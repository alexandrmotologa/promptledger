package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/api"
	"github.com/alexandrmotologa/promptledger/internal/cache"
	"github.com/alexandrmotologa/promptledger/internal/eval"
	"github.com/alexandrmotologa/promptledger/internal/evaluator"
	"github.com/alexandrmotologa/promptledger/internal/gate"
	"github.com/alexandrmotologa/promptledger/internal/provider"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/router"
	"github.com/alexandrmotologa/promptledger/internal/store"
	"github.com/alexandrmotologa/promptledger/web"
	"github.com/spf13/cobra"
)

var (
	promptsDir string
	dataDir    string
	port       int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "promptledger",
		Short: "promptledger — Git-Native Prompt Version Control, A/B Traffic Router & Shadow Evaluator",
	}

	rootCmd.PersistentFlags().StringVarP(&promptsDir, "prompts", "p", "./prompts", "Path to prompts YAML directory")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data", "d", ".promptledger", "Path to data directory for SQLite store")

	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(promoteCmd())
	rootCmd.AddCommand(rollbackCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(diffCmd())
	rootCmd.AddCommand(evalCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start promptledger proxy router daemon and embedded experimentation studio",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("================================================================================")
			fmt.Println("  promptledger — Git-Native Prompt Version Control & Traffic Router")
			fmt.Println("================================================================================")

			// 1. Initialize Registry
			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return fmt.Errorf("failed to load registry: %w", err)
			}
			if err := reg.StartWatcher(); err != nil {
				fmt.Printf("[warning] fsnotify watcher disabled: %v\n", err)
			}
			defer reg.Close()

			fmt.Printf("✓ Loaded %d prompt specifications from %s\n", len(reg.ListPrompts()), promptsDir)

			// 2. Initialize Store
			dbPath := filepath.Join(dataDir, "promptledger.db")
			s, err := store.NewStore(dbPath)
			if err != nil {
				return fmt.Errorf("failed to initialize sqlite store: %w", err)
			}
			defer s.Close()
			fmt.Printf("✓ Initialized SQLite experimentation store (WAL mode) at %s\n", dbPath)

			// 3. Components
			r := router.NewRouter(reg)
			p := provider.NewClient()
			evalPool := evaluator.NewEvaluator(s, p, 4, 500)
			defer evalPool.Stop()

			gm := gate.NewGateManager(reg, s)
			c := cache.NewCache()

			// 4. Embedded static UI
			staticFS, err := web.GetFS()
			if err != nil {
				fmt.Printf("[warning] could not load embedded web filesystem: %v\n", err)
			}

			srv := api.NewServer(reg, r, p, s, evalPool, gm, c, staticFS)

			addr := fmt.Sprintf(":%d", port)
			httpServer := &http.Server{
				Addr:         addr,
				Handler:      srv,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}

			go func() {
				fmt.Printf("\n🚀 Daemon listening on http://127.0.0.1:%d\n", port)
				fmt.Printf("📊 React 19 Experimentation Studio: http://127.0.0.1:%d/\n", port)
				fmt.Printf("⚡ Proxy Router Endpoint: POST http://127.0.0.1:%d/v1/prompts/{prompt_id}/execute\n", port)
				fmt.Printf("📈 Prometheus Metrics Endpoint: GET http://127.0.0.1:%d/metrics\n\n", port)
				if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Printf("HTTP server error: %v\n", err)
				}
			}()

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
			<-quit

			fmt.Println("\nShutting down promptledger gracefully...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return httpServer.Shutdown(ctx)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "P", 8080, "HTTP server port")
	return cmd
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [directory or file]",
		Short: "Validate prompt YAML specs, template variables, assertions, and routing configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetDir := promptsDir
			if len(args) > 0 {
				targetDir = args[0]
			}

			info, err := os.Stat(targetDir)
			if err != nil {
				return err
			}

			var files []string
			if info.IsDir() {
				entries, err := os.ReadDir(targetDir)
				if err != nil {
					return err
				}
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
						files = append(files, filepath.Join(targetDir, e.Name()))
					}
				}
			} else {
				files = append(files, targetDir)
			}

			fmt.Printf("Validating %d prompt specification files...\n", len(files))
			hasErrors := false

			for _, path := range files {
				spec, err := registry.LoadPromptFile(path)
				if err != nil {
					fmt.Printf("❌ %s: %v\n", filepath.Base(path), err)
					hasErrors = true
					continue
				}

				totalVars := 0
				for _, vDetail := range spec.Versions {
					vVars := registry.ExtractTemplateVariables(vDetail.Template)
					totalVars += len(vVars)
				}

				strategy := spec.Routing.Strategy
				if spec.Routing.Bandit.Enabled {
					strategy += " (bandit-enabled)"
				}

				assertionsCount := len(spec.Evaluations)
				caching := "off"
				if spec.Cache.Enabled {
					caching = fmt.Sprintf("on (%ds TTL)", spec.Cache.TTLSeconds)
				}

				fmt.Printf("✓ %s (Prompt ID: %s, Strategy: %s, %d versions, %d vars, %d assertions, cache: %s)\n",
					filepath.Base(path), spec.PromptID, strategy, len(spec.Versions), totalVars, assertionsCount, caching)
			}

			if hasErrors {
				return fmt.Errorf("one or more prompt specifications failed validation")
			}
			fmt.Println("\nAll prompt specifications are valid and ready for production routing.")
			return nil
		},
	}
}

func evalCmd() *cobra.Command {
	var promptID string
	var version string
	var datasetPath string

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run offline benchmark on a golden JSONL dataset for CI/CD pre-merge validation",
		RunE: func(cmd *cobra.Command, args []string) error {
			if promptID == "" {
				return fmt.Errorf("--prompt flag is required")
			}
			if datasetPath == "" {
				datasetPath = "evals/checkout_intent_eval.jsonl"
			}

			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return err
			}

			spec, ok := reg.GetPrompt(promptID)
			if !ok {
				return fmt.Errorf("prompt '%s' not found", promptID)
			}

			targetVer := version
			if targetVer == "" {
				targetVer = spec.Routing.ActiveVersion
			}

			prov := provider.NewClient()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			fmt.Printf("Running golden evaluation on '%s' (%s) using dataset '%s'...\n\n", promptID, targetVer, datasetPath)
			report, err := eval.RunEvaluation(ctx, spec, targetVer, datasetPath, prov)
			if err != nil {
				return err
			}

			fmt.Print(report.MarkdownSummary)

			if report.AccuracyRate < spec.Gates.MinQualityScore && spec.Gates.MinQualityScore > 0 {
				return fmt.Errorf("evaluation failed: accuracy %.1f%% below minimum quality gate %.1f%%",
					report.AccuracyRate*100, spec.Gates.MinQualityScore*100)
			}

			fmt.Printf("\n✓ Evaluation PASSED (Quality gate: ≥%.1f%%)\n", spec.Gates.MinQualityScore*100)
			return nil
		},
	}

	cmd.Flags().StringVar(&promptID, "prompt", "", "Prompt ID to evaluate")
	cmd.Flags().StringVar(&version, "version", "", "Version to evaluate (defaults to active)")
	cmd.Flags().StringVar(&datasetPath, "dataset", "evals/checkout_intent_eval.jsonl", "Path to JSONL golden test dataset")
	return cmd
}

func promoteCmd() *cobra.Command {
	var promptID string
	var version string

	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a canary prompt version to 100% active traffic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if promptID == "" || version == "" {
				return fmt.Errorf("--prompt and --version flags are required")
			}

			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return err
			}

			dbPath := filepath.Join(dataDir, "promptledger.db")
			s, err := store.NewStore(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			gm := gate.NewGateManager(reg, s)
			if err := gm.Promote(promptID, version, "cli"); err != nil {
				return err
			}

			fmt.Printf("✓ Successfully promoted '%s' to version '%s' (100%% active traffic).\n", promptID, version)
			return nil
		},
	}

	cmd.Flags().StringVar(&promptID, "prompt", "", "Prompt ID to promote")
	cmd.Flags().StringVar(&version, "version", "", "Target version to promote")
	return cmd
}

func rollbackCmd() *cobra.Command {
	var promptID string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back a prompt to its previous stable version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if promptID == "" {
				return fmt.Errorf("--prompt flag is required")
			}

			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return err
			}

			dbPath := filepath.Join(dataDir, "promptledger.db")
			s, err := store.NewStore(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			gm := gate.NewGateManager(reg, s)
			if err := gm.Rollback(promptID, "cli"); err != nil {
				return err
			}

			spec, _ := reg.GetPrompt(promptID)
			fmt.Printf("✓ Successfully rolled back '%s' to stable version '%s'.\n", promptID, spec.Routing.ActiveVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&promptID, "prompt", "", "Prompt ID to roll back")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Inspect registered prompts, active traffic splits, caching, and shadow mode status",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return err
			}

			prompts := reg.ListPrompts()
			fmt.Printf("\n%-28s | %-10s | %-18s | %-10s | %-22s\n", "PROMPT ID", "ACTIVE VER", "STRATEGY / SPLIT", "CACHE", "SHADOW EVALUATION")
			fmt.Println(strings.Repeat("-", 100))

			for _, p := range prompts {
				var splits []string
				for _, v := range p.Routing.Variants {
					splits = append(splits, fmt.Sprintf("%s (%d%%)", v.Version, v.Weight))
				}
				splitStr := strings.Join(splits, ", ")
				if p.Routing.Strategy == "bandit" || p.Routing.Bandit.Enabled {
					splitStr = "Bandit Adaptive"
				}

				cacheStr := "Off"
				if p.Cache.Enabled {
					cacheStr = fmt.Sprintf("On (%ds)", p.Cache.TTLSeconds)
				}

				shadowStr := "Disabled"
				if p.Routing.Shadow.Enabled {
					shadowStr = fmt.Sprintf("%s (%.0f%% sample)", p.Routing.Shadow.Version, p.Routing.Shadow.SampleRate*100)
				}

				fmt.Printf("%-28s | %-10s | %-18s | %-10s | %-22s\n", p.PromptID, p.Routing.ActiveVersion, splitStr, cacheStr, shadowStr)
			}
			fmt.Println()
			return nil
		},
	}
}

func diffCmd() *cobra.Command {
	var promptID string
	var v1, v2 string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare prompt templates between two versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if promptID == "" || v1 == "" || v2 == "" {
				return fmt.Errorf("--prompt, --v1, and --v2 are required")
			}

			reg, err := registry.NewRegistry(promptsDir)
			if err != nil {
				return err
			}

			spec, ok := reg.GetPrompt(promptID)
			if !ok {
				return fmt.Errorf("prompt '%s' not found", promptID)
			}

			d1, ok1 := spec.Versions[v1]
			d2, ok2 := spec.Versions[v2]
			if !ok1 || !ok2 {
				return fmt.Errorf("version '%s' or '%s' not found in prompt", v1, v2)
			}

			fmt.Printf("=== Prompt Diff: %s [%s vs %s] ===\n\n", promptID, v1, v2)
			fmt.Printf("Version %s (Model: %s, Author: %s):\n", v1, d1.Model, d1.Author)
			fmt.Println(strings.Repeat("-", 60))
			fmt.Println(d1.Template)
			fmt.Println(strings.Repeat("-", 60))

			fmt.Printf("\nVersion %s (Model: %s, Author: %s):\n", v2, d2.Model, d2.Author)
			fmt.Println(strings.Repeat("-", 60))
			fmt.Println(d2.Template)
			fmt.Println(strings.Repeat("-", 60))

			return nil
		},
	}

	cmd.Flags().StringVar(&promptID, "prompt", "", "Prompt ID")
	cmd.Flags().StringVar(&v1, "v1", "", "First version")
	cmd.Flags().StringVar(&v2, "v2", "", "Second version")
	return cmd
}
