package registry

import "time"

// VariantConfig defines a version and its routing weight (0-100).
type VariantConfig struct {
	Version string `yaml:"version" json:"version"`
	Weight  int    `yaml:"weight" json:"weight"`
}

// ShadowConfig defines background shadow execution parameters.
type ShadowConfig struct {
	Enabled    bool    `yaml:"enabled" json:"enabled"`
	Version    string  `yaml:"version" json:"version"`
	SampleRate float64 `yaml:"sample_rate" json:"sample_rate"` // e.g. 0.05 for 5%
}

// RoutingConfig defines traffic distribution across prompt versions.
type RoutingConfig struct {
	Strategy      string          `yaml:"strategy" json:"strategy"` // canary, static, ab_split
	ActiveVersion string          `yaml:"active_version" json:"active_version"`
	Variants      []VariantConfig `yaml:"variants" json:"variants"`
	Shadow        ShadowConfig    `yaml:"shadow" json:"shadow"`
}

// GateConfig defines automated threshold gates for promotion and rollback.
type GateConfig struct {
	MinQualityScore float64 `yaml:"min_quality_score" json:"min_quality_score"` // 0.0 - 1.0
	MaxP95LatencyMs int64   `yaml:"max_p95_latency_ms" json:"max_p95_latency_ms"` // ms
	MaxErrorRate    float64 `yaml:"max_error_rate" json:"max_error_rate"`       // 0.0 - 1.0 (e.g. 0.01)
	MinSampleSize   int     `yaml:"min_sample_size" json:"min_sample_size"`     // min runs before gate evaluation
	AutoPromote     bool    `yaml:"auto_promote" json:"auto_promote"`
	AutoRollback    bool    `yaml:"auto_rollback" json:"auto_rollback"`
}

// VersionDetail holds the LLM model target, prompt template, author, and Git metadata.
type VersionDetail struct {
	Model       string            `yaml:"model" json:"model"`
	Template    string            `yaml:"template" json:"template"`
	Author      string            `yaml:"author" json:"author"`
	Commit      string            `yaml:"commit" json:"commit"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Parameters  map[string]string `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	CreatedAt   time.Time         `yaml:"created_at" json:"created_at"`
}

// PromptSpec represents the declarative, Git-native prompt definition.
type PromptSpec struct {
	Version     string                    `yaml:"version" json:"version"`
	PromptID    string                    `yaml:"prompt_id" json:"prompt_id"`
	Description string                    `yaml:"description" json:"description"`
	Routing     RoutingConfig             `yaml:"routing" json:"routing"`
	Gates       GateConfig                `yaml:"gates" json:"gates"`
	Versions    map[string]*VersionDetail `yaml:"versions" json:"versions"`
	FilePath    string                    `yaml:"-" json:"file_path,omitempty"`
	UpdatedAt   time.Time                 `yaml:"-" json:"updated_at"`
}
