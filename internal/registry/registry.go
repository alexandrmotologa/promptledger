package registry

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Registry manages the prompt specifications loaded from disk.
type Registry struct {
	mu        sync.RWMutex
	dir       string
	prompts   map[string]*PromptSpec
	watcher   *fsnotify.Watcher
	stopChan  chan struct{}
	listeners []func(promptID string, spec *PromptSpec)
}

// NewRegistry initializes and populates a prompt registry from a directory.
func NewRegistry(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prompts dir: %w", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	r := &Registry{
		dir:      absDir,
		prompts:  make(map[string]*PromptSpec),
		stopChan: make(chan struct{}),
	}

	if err := r.ReloadAll(); err != nil {
		return nil, err
	}

	return r, nil
}

// StartWatcher begins watching the prompt directory for file additions/edits.
func (r *Registry) StartWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to initialize fsnotify: %w", err)
	}
	r.watcher = watcher

	if err := watcher.Add(r.dir); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("failed to watch directory %s: %w", r.dir, err)
	}

	go r.watchLoop()
	return nil
}

// Close gracefully stops file watching.
func (r *Registry) Close() error {
	close(r.stopChan)
	if r.watcher != nil {
		return r.watcher.Close()
	}
	return nil
}

func (r *Registry) watchLoop() {
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	for {
		select {
		case <-r.stopChan:
			return
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			if strings.HasSuffix(event.Name, ".yaml") || strings.HasSuffix(event.Name, ".yml") {
				debounceMu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
					_ = r.ReloadAll()
				})
				debounceMu.Unlock()
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("[registry watcher error] %v\n", err)
		}
	}
}

// ReloadAll scans the prompts directory and reloads all YAML prompt specifications.
func (r *Registry) ReloadAll() error {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	newPrompts := make(map[string]*PromptSpec)

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		fullPath := filepath.Join(r.dir, f.Name())
		spec, err := LoadPromptFile(fullPath)
		if err != nil {
			fmt.Printf("[registry warning] skipping invalid prompt file %s: %v\n", f.Name(), err)
			continue
		}

		newPrompts[spec.PromptID] = spec
	}

	r.mu.Lock()
	r.prompts = newPrompts
	r.mu.Unlock()

	return nil
}

// LoadPromptFile loads and parses a single YAML prompt definition file.
func LoadPromptFile(path string) (*PromptSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var spec PromptSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}

	if err := ValidatePromptSpec(&spec); err != nil {
		return nil, err
	}

	spec.FilePath = path
	spec.UpdatedAt = time.Now()

	return &spec, nil
}

// ValidatePromptSpec performs strict sanity checks on the prompt structure.
func ValidatePromptSpec(spec *PromptSpec) error {
	if strings.TrimSpace(spec.PromptID) == "" {
		return fmt.Errorf("prompt_id is required")
	}

	if len(spec.Versions) == 0 {
		return fmt.Errorf("at least one version must be defined in versions")
	}

	if spec.Routing.ActiveVersion == "" {
		return fmt.Errorf("routing.active_version is required")
	}

	if _, exists := spec.Versions[spec.Routing.ActiveVersion]; !exists {
		return fmt.Errorf("routing.active_version '%s' is not defined in versions", spec.Routing.ActiveVersion)
	}

	// Validate variants weight
	if len(spec.Routing.Variants) > 0 {
		totalWeight := 0
		for _, v := range spec.Routing.Variants {
			if _, exists := spec.Versions[v.Version]; !exists {
				return fmt.Errorf("variant version '%s' is not defined in versions", v.Version)
			}
			if v.Weight < 0 || v.Weight > 100 {
				return fmt.Errorf("variant weight must be between 0 and 100, got %d", v.Weight)
			}
			totalWeight += v.Weight
		}
		if totalWeight != 100 {
			return fmt.Errorf("variant weights must sum to 100, got %d", totalWeight)
		}
	} else {
		// Default single active variant with 100% weight
		spec.Routing.Variants = []VariantConfig{
			{Version: spec.Routing.ActiveVersion, Weight: 100},
		}
	}

	// Validate shadow configuration
	if spec.Routing.Shadow.Enabled {
		if spec.Routing.Shadow.Version == "" {
			return fmt.Errorf("routing.shadow.version is required when shadow is enabled")
		}
		if _, exists := spec.Versions[spec.Routing.Shadow.Version]; !exists {
			return fmt.Errorf("routing.shadow.version '%s' is not defined in versions", spec.Routing.Shadow.Version)
		}
		if spec.Routing.Shadow.SampleRate <= 0.0 || spec.Routing.Shadow.SampleRate > 1.0 {
			return fmt.Errorf("routing.shadow.sample_rate must be between 0.0 and 1.0, got %f", spec.Routing.Shadow.SampleRate)
		}
	}

	// Validate templates
	for vName, vDetail := range spec.Versions {
		if strings.TrimSpace(vDetail.Template) == "" {
			return fmt.Errorf("template for version '%s' cannot be empty", vName)
		}
		if _, err := RenderTemplate(vDetail.Template, map[string]interface{}{}); err != nil {
			// Test compilation without error on missing keys
			norm := NormalizeTemplate(vDetail.Template)
			if _, parseErr := RenderTemplate(norm, map[string]interface{}{"UserInput": "test", "Context": "test"}); parseErr != nil {
				// Warn or check parsing
			}
		}
	}

	return nil
}

// GetPrompt retrieves a prompt specification by ID.
func (r *Registry) GetPrompt(promptID string) (*PromptSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.prompts[promptID]
	return p, ok
}

// ListPrompts returns all loaded prompt specifications.
func (r *Registry) ListPrompts() []*PromptSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*PromptSpec, 0, len(r.prompts))
	for _, p := range r.prompts {
		list = append(list, p)
	}
	return list
}

// SavePrompt writes the prompt specification to disk and updates in-memory registry.
func (r *Registry) SavePrompt(spec *PromptSpec) error {
	if err := ValidatePromptSpec(spec); err != nil {
		return err
	}

	filePath := spec.FilePath
	if filePath == "" {
		filePath = filepath.Join(r.dir, spec.PromptID+".yaml")
		spec.FilePath = filePath
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	spec.UpdatedAt = time.Now()

	r.mu.Lock()
	r.prompts[spec.PromptID] = spec
	r.mu.Unlock()

	return nil
}

// GetGitMetadata returns current git commit hash and author if inside a repository.
func GetGitMetadata(dir string) (commit string, author string) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	} else {
		commit = "local-dev"
	}

	cmdAuthor := exec.Command("git", "config", "user.name")
	cmdAuthor.Dir = dir
	if out, err := cmdAuthor.Output(); err == nil && len(out) > 0 {
		author = strings.TrimSpace(string(out))
	} else {
		author = "engineer@promptledger.dev"
	}

	return commit, author
}
