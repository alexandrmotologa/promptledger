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

// BanditConfig defines Multi-Armed Bandit adaptive traffic routing parameters.
type BanditConfig struct {
	Enabled         bool    `yaml:"enabled" json:"enabled"`
	Epsilon         float64 `yaml:"epsilon" json:"epsilon"`                   // Exploration rate (e.g. 0.10)
	QualityWeight   float64 `yaml:"quality_weight" json:"quality_weight"`     // e.g. 0.50
	LatencyWeight   float64 `yaml:"latency_weight" json:"latency_weight"`     // e.g. 0.25
	CostWeight      float64 `yaml:"cost_weight" json:"cost_weight"`           // e.g. 0.25
	MinRunsToExploit int    `yaml:"min_runs_to_exploit" json:"min_runs_to_exploit"` // Min samples per arm before exploiting
}

// RoutingConfig defines traffic distribution across prompt versions.
type RoutingConfig struct {
	Strategy        string          `yaml:"strategy" json:"strategy"` // canary, static, bandit, ab_split
	ActiveVersion   string          `yaml:"active_version" json:"active_version"`
	Variants        []VariantConfig `yaml:"variants" json:"variants"`
	Shadow          ShadowConfig    `yaml:"shadow" json:"shadow"`
	Bandit          BanditConfig    `yaml:"bandit" json:"bandit"`
	FallbackVersion string          `yaml:"fallback_version,omitempty" json:"fallback_version,omitempty"`
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

// AssertionSpec defines declarative semantic and structural quality assertions.
type AssertionSpec struct {
	Type        string   `yaml:"type" json:"type"` // json_schema, regex, banned_words, llm_judge
	Pattern     string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	BannedWords []string `yaml:"banned_words,omitempty" json:"banned_words,omitempty"`
	Rubric      string   `yaml:"rubric,omitempty" json:"rubric,omitempty"`
	Threshold   float64  `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

// BudgetConfig defines spending guardrails and automated model downscaling.
type BudgetConfig struct {
	MaxDailyCostUSD float64 `yaml:"max_daily_cost_usd" json:"max_daily_cost_usd"`
	FallbackModel   string  `yaml:"fallback_model" json:"fallback_model"` // e.g. gpt-4o-mini or gemini-2.0-flash
}

// CacheConfig defines local semantic caching settings.
type CacheConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	TTLSeconds int  `yaml:"ttl_seconds" json:"ttl_seconds"` // e.g. 3600 (1 hour)
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
	Evaluations []AssertionSpec           `yaml:"evaluations,omitempty" json:"evaluations,omitempty"`
	Budget      BudgetConfig              `yaml:"budget,omitempty" json:"budget,omitempty"`
	Cache       CacheConfig               `yaml:"cache,omitempty" json:"cache,omitempty"`
	Versions    map[string]*VersionDetail `yaml:"versions" json:"versions"`
	FilePath    string                    `yaml:"-" json:"file_path,omitempty"`
	UpdatedAt   time.Time                 `yaml:"-" json:"updated_at"`
}
