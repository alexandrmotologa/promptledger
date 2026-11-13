package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ExecutionRecord captures an individual prompt evaluation run.
type ExecutionRecord struct {
	ID               string                 `json:"id"`
	PromptID         string                 `json:"prompt_id"`
	Version          string                 `json:"version"`
	VariantType      string                 `json:"variant_type"` // active, canary, shadow
	UserID           string                 `json:"user_id"`
	TenantID         string                 `json:"tenant_id"`
	InputVariables   map[string]interface{} `json:"input_variables"`
	RenderedPrompt   string                 `json:"rendered_prompt"`
	OutputText       string                 `json:"output_text"`
	ModelUsed        string                 `json:"model_used"`
	LatencyMs        int64                  `json:"latency_ms"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	TotalTokens      int                    `json:"total_tokens"`
	CostUSD          float64                `json:"cost_usd"`
	IsSuccess        bool                   `json:"is_success"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// ShadowComparisonRecord stores parallel quality and latency evaluation.
type ShadowComparisonRecord struct {
	ID                 int64     `json:"id"`
	PromptID           string    `json:"prompt_id"`
	ActiveExecutionID  string    `json:"active_execution_id"`
	ShadowExecutionID  string    `json:"shadow_execution_id"`
	ActiveVersion      string    `json:"active_version"`
	ShadowVersion      string    `json:"shadow_version"`
	QualityScore       float64   `json:"quality_score"` // 0.0 - 1.0
	LatencyDeltaMs     int64     `json:"latency_delta_ms"`
	CostDeltaUSD       float64   `json:"cost_delta_usd"`
	SemanticSimilarity float64   `json:"semantic_similarity"`
	Verdict            string    `json:"verdict"` // better, equivalent, regressed
	Explanation        string    `json:"explanation"`
	CreatedAt          time.Time `json:"created_at"`

	// Enriched fields for visual studio inspector
	ActiveOutput string `json:"active_output,omitempty"`
	ShadowOutput string `json:"shadow_output,omitempty"`
	InputJSON    string `json:"input_json,omitempty"`
}

// AuditEventRecord tracks administrative promotions, rollbacks, and policy gates.
type AuditEventRecord struct {
	ID           int64     `json:"id"`
	PromptID     string    `json:"prompt_id"`
	Action       string    `json:"action"` // promote, rollback, weight_change, shadow_toggle
	OldVersion   string    `json:"old_version"`
	NewVersion   string    `json:"new_version"`
	Reason       string    `json:"reason"`
	TriggeredBy  string    `json:"triggered_by"` // auto_gate, cli, studio_user
	MetadataJSON string    `json:"metadata_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// VersionMetric summarizes aggregated stats for a specific version.
type VersionMetric struct {
	Version          string  `json:"version"`
	VariantType      string  `json:"variant_type"`
	TotalRuns        int     `json:"total_runs"`
	ErrorCount       int     `json:"error_count"`
	ErrorRate        float64 `json:"error_rate"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P95LatencyMs     int64   `json:"p95_latency_ms"`
	AvgCostUSD       float64 `json:"avg_cost_usd"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	AvgQualityScore  float64 `json:"avg_quality_score"`
	SemanticMatchPct float64 `json:"semantic_match_pct"`
}

// PromptMetrics represents comprehensive analytics for a prompt.
type PromptMetrics struct {
	PromptID             string                   `json:"prompt_id"`
	ActiveVersion        string                   `json:"active_version"`
	TotalRuns            int                      `json:"total_runs"`
	Versions             map[string]*VersionMetric `json:"versions"`
	ShadowComparisons    int                      `json:"shadow_comparisons"`
	AvgShadowAgreement   float64                  `json:"avg_shadow_agreement"`
	AvgLatencyDeltaMs    int64                    `json:"avg_latency_delta_ms"`
	EstMonthlyCostDelta  float64                  `json:"est_monthly_cost_delta"`
	AutoPromoteCandidate bool                     `json:"auto_promote_candidate"`
	RollbackWarning      bool                     `json:"rollback_warning"`
}

// SystemSummary gives platform-level observability stats.
type SystemSummary struct {
	TotalPrompts       int     `json:"total_prompts"`
	TotalExecutions    int     `json:"total_executions"`
	TotalShadowRuns    int     `json:"total_shadow_runs"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	TotalCostUSD       float64 `json:"total_cost_usd"`
	AvgShadowAgreement float64 `json:"avg_shadow_agreement"`
	ActiveCanaries     int     `json:"active_canaries"`
}

// Store handles SQLite persistence for runtime analytics.
type Store struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewStore initializes the SQLite database with WAL mode and tables.
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure WAL mode and connection pooling
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA temp_store=MEMORY;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("failed executing pragma %s: %w", p, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close gracefully closes the database handle.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS executions (
		id TEXT PRIMARY KEY,
		prompt_id TEXT NOT NULL,
		version TEXT NOT NULL,
		variant_type TEXT NOT NULL,
		user_id TEXT,
		tenant_id TEXT,
		input_variables TEXT,
		rendered_prompt TEXT,
		output_text TEXT,
		model_used TEXT,
		latency_ms INTEGER NOT NULL,
		prompt_tokens INTEGER NOT NULL,
		completion_tokens INTEGER NOT NULL,
		total_tokens INTEGER NOT NULL,
		cost_usd REAL NOT NULL,
		is_success INTEGER NOT NULL,
		error_message TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_exec_prompt_ver ON executions(prompt_id, version);
	CREATE INDEX IF NOT EXISTS idx_exec_created_at ON executions(created_at DESC);

	CREATE TABLE IF NOT EXISTS shadow_comparisons (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt_id TEXT NOT NULL,
		active_execution_id TEXT NOT NULL,
		shadow_execution_id TEXT NOT NULL,
		active_version TEXT NOT NULL,
		shadow_version TEXT NOT NULL,
		quality_score REAL NOT NULL,
		latency_delta_ms INTEGER NOT NULL,
		cost_delta_usd REAL NOT NULL,
		semantic_similarity REAL NOT NULL,
		verdict TEXT NOT NULL,
		explanation TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_shadow_prompt ON shadow_comparisons(prompt_id);
	CREATE INDEX IF NOT EXISTS idx_shadow_created ON shadow_comparisons(created_at DESC);

	CREATE TABLE IF NOT EXISTS audit_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt_id TEXT NOT NULL,
		action TEXT NOT NULL,
		old_version TEXT,
		new_version TEXT,
		reason TEXT,
		triggered_by TEXT NOT NULL,
		metadata_json TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_audit_prompt ON audit_events(prompt_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// RecordExecution persists an individual execution record.
func (s *Store) RecordExecution(rec *ExecutionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var inputJSON string
	if rec.InputVariables != nil {
		if b, err := json.Marshal(rec.InputVariables); err == nil {
			inputJSON = string(b)
		}
	}

	isSuccessInt := 0
	if rec.IsSuccess {
		isSuccessInt = 1
	}

	query := `
	INSERT INTO executions (
		id, prompt_id, version, variant_type, user_id, tenant_id,
		input_variables, rendered_prompt, output_text, model_used,
		latency_ms, prompt_tokens, completion_tokens, total_tokens,
		cost_usd, is_success, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err := s.db.Exec(query,
		rec.ID, rec.PromptID, rec.Version, rec.VariantType, rec.UserID, rec.TenantID,
		inputJSON, rec.RenderedPrompt, rec.OutputText, rec.ModelUsed,
		rec.LatencyMs, rec.PromptTokens, rec.CompletionTokens, rec.TotalTokens,
		rec.CostUSD, isSuccessInt, rec.ErrorMessage, rec.CreatedAt.Format(time.RFC3339Nano),
	)
	return err
}

// RecordShadowComparison stores quality, similarity, and latency metrics for shadow runs.
func (s *Store) RecordShadowComparison(rec *ShadowComparisonRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO shadow_comparisons (
		prompt_id, active_execution_id, shadow_execution_id,
		active_version, shadow_version, quality_score, latency_delta_ms,
		cost_delta_usd, semantic_similarity, verdict, explanation, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	res, err := s.db.Exec(query,
		rec.PromptID, rec.ActiveExecutionID, rec.ShadowExecutionID,
		rec.ActiveVersion, rec.ShadowVersion, rec.QualityScore, rec.LatencyDeltaMs,
		rec.CostDeltaUSD, rec.SemanticSimilarity, rec.Verdict, rec.Explanation,
		rec.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err == nil {
		rec.ID = id
	}
	return nil
}

// RecordAuditEvent stores an operational change or automated gate trigger.
func (s *Store) RecordAuditEvent(rec *AuditEventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO audit_events (
		prompt_id, action, old_version, new_version, reason, triggered_by, metadata_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`

	res, err := s.db.Exec(query,
		rec.PromptID, rec.Action, rec.OldVersion, rec.NewVersion,
		rec.Reason, rec.TriggeredBy, rec.MetadataJSON, rec.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err == nil {
		rec.ID = id
	}
	return nil
}

// GetRecentExecutions retrieves recent execution records for a prompt.
func (s *Store) GetRecentExecutions(promptID string, limit int) ([]*ExecutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if promptID != "" {
		rows, err = s.db.Query(`
			SELECT id, prompt_id, version, variant_type, user_id, tenant_id,
				   input_variables, rendered_prompt, output_text, model_used,
				   latency_ms, prompt_tokens, completion_tokens, total_tokens,
				   cost_usd, is_success, error_message, created_at
			FROM executions
			WHERE prompt_id = ?
			ORDER BY created_at DESC
			LIMIT ?;
		`, promptID, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, prompt_id, version, variant_type, user_id, tenant_id,
				   input_variables, rendered_prompt, output_text, model_used,
				   latency_ms, prompt_tokens, completion_tokens, total_tokens,
				   cost_usd, is_success, error_message, created_at
			FROM executions
			ORDER BY created_at DESC
			LIMIT ?;
		`, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ExecutionRecord
	for rows.Next() {
		var r ExecutionRecord
		var inputJSON, createdAtStr string
		var isSuccessInt int

		err := rows.Scan(
			&r.ID, &r.PromptID, &r.Version, &r.VariantType, &r.UserID, &r.TenantID,
			&inputJSON, &r.RenderedPrompt, &r.OutputText, &r.ModelUsed,
			&r.LatencyMs, &r.PromptTokens, &r.CompletionTokens, &r.TotalTokens,
			&r.CostUSD, &isSuccessInt, &r.ErrorMessage, &createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		r.IsSuccess = isSuccessInt == 1
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		if inputJSON != "" {
			_ = json.Unmarshal([]byte(inputJSON), &r.InputVariables)
		}

		records = append(records, &r)
	}

	return records, nil
}

// GetRecentShadowComparisons retrieves paired execution diffs with enriched payloads.
func (s *Store) GetRecentShadowComparisons(promptID string, limit int) ([]*ShadowComparisonRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT s.id, s.prompt_id, s.active_execution_id, s.shadow_execution_id,
		       s.active_version, s.shadow_version, s.quality_score, s.latency_delta_ms,
		       s.cost_delta_usd, s.semantic_similarity, s.verdict, s.explanation, s.created_at,
		       COALESCE(a.output_text, '') AS active_output,
		       COALESCE(sh.output_text, '') AS shadow_output,
		       COALESCE(a.input_variables, '') AS input_json
		FROM shadow_comparisons s
		LEFT JOIN executions a ON s.active_execution_id = a.id
		LEFT JOIN executions sh ON s.shadow_execution_id = sh.id
	`
	var rows *sql.Rows
	var err error

	if promptID != "" {
		query += " WHERE s.prompt_id = ? ORDER BY s.created_at DESC LIMIT ?;"
		rows, err = s.db.Query(query, promptID, limit)
	} else {
		query += " ORDER BY s.created_at DESC LIMIT ?;"
		rows, err = s.db.Query(query, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ShadowComparisonRecord
	for rows.Next() {
		var r ShadowComparisonRecord
		var createdAtStr string

		err := rows.Scan(
			&r.ID, &r.PromptID, &r.ActiveExecutionID, &r.ShadowExecutionID,
			&r.ActiveVersion, &r.ShadowVersion, &r.QualityScore, &r.LatencyDeltaMs,
			&r.CostDeltaUSD, &r.SemanticSimilarity, &r.Verdict, &r.Explanation, &createdAtStr,
			&r.ActiveOutput, &r.ShadowOutput, &r.InputJSON,
		)
		if err != nil {
			return nil, err
		}

		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		records = append(records, &r)
	}

	return records, nil
}

// GetPromptMetrics computes comprehensive statistical analysis by version.
func (s *Store) GetPromptMetrics(promptID string) (*PromptMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := &PromptMetrics{
		PromptID: promptID,
		Versions: make(map[string]*VersionMetric),
	}

	rows, err := s.db.Query(`
		SELECT version, variant_type, latency_ms, cost_usd, is_success
		FROM executions
		WHERE prompt_id = ?;
	`, promptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	latenciesByVer := make(map[string][]int64)

	for rows.Next() {
		var ver, vType string
		var lat int64
		var cost float64
		var isSuccess int

		if err := rows.Scan(&ver, &vType, &lat, &cost, &isSuccess); err != nil {
			return nil, err
		}

		vm, exists := metrics.Versions[ver]
		if !exists {
			vm = &VersionMetric{
				Version:     ver,
				VariantType: vType,
			}
			metrics.Versions[ver] = vm
		}

		vm.TotalRuns++
		if isSuccess == 0 {
			vm.ErrorCount++
		}
		vm.TotalCostUSD += cost
		latenciesByVer[ver] = append(latenciesByVer[ver], lat)
		metrics.TotalRuns++
	}

	// Calculate P95, averages, error rates
	for ver, vm := range metrics.Versions {
		if vm.TotalRuns > 0 {
			vm.ErrorRate = float64(vm.ErrorCount) / float64(vm.TotalRuns)
			vm.AvgCostUSD = vm.TotalCostUSD / float64(vm.TotalRuns)

			lats := latenciesByVer[ver]
			sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

			var sumLat int64
			for _, l := range lats {
				sumLat += l
			}
			vm.AvgLatencyMs = float64(sumLat) / float64(len(lats))

			p95Idx := int(float64(len(lats)) * 0.95)
			if p95Idx >= len(lats) {
				p95Idx = len(lats) - 1
			}
			if len(lats) > 0 {
				vm.P95LatencyMs = lats[p95Idx]
			}
		}
	}

	// Query shadow comparisons for quality and latency deltas
	sRows, err := s.db.Query(`
		SELECT quality_score, latency_delta_ms, cost_delta_usd, semantic_similarity
		FROM shadow_comparisons
		WHERE prompt_id = ?;
	`, promptID)
	if err == nil {
		defer sRows.Close()
		var totalSim, totalQuality, totalCostDelta float64
		var totalLatDelta int64
		var sCount int

		for sRows.Next() {
			var q, sim, costD float64
			var latD int64
			if err := sRows.Scan(&q, &latD, &costD, &sim); err == nil {
				totalQuality += q
				totalSim += sim
				totalLatDelta += latD
				totalCostDelta += costD
				sCount++
			}
		}

		if sCount > 0 {
			metrics.ShadowComparisons = sCount
			metrics.AvgShadowAgreement = totalQuality / float64(sCount)
			metrics.AvgLatencyDeltaMs = totalLatDelta / int64(sCount)
			metrics.EstMonthlyCostDelta = (totalCostDelta / float64(sCount)) * 100000.0 // based on 100k req/mo baseline
		}
	}

	return metrics, nil
}

// GetSystemSummary aggregates statistics across all registered prompts.
func (s *Store) GetSystemSummary() (*SystemSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &SystemSummary{}

	// Total prompts
	_ = s.db.QueryRow(`SELECT count(DISTINCT prompt_id) FROM executions;`).Scan(&summary.TotalPrompts)

	// Total executions, avg latency, total cost
	_ = s.db.QueryRow(`
		SELECT count(*), COALESCE(avg(latency_ms), 0), COALESCE(sum(cost_usd), 0)
		FROM executions;
	`).Scan(&summary.TotalExecutions, &summary.AvgLatencyMs, &summary.TotalCostUSD)

	// Total shadow runs & average quality
	_ = s.db.QueryRow(`
		SELECT count(*), COALESCE(avg(quality_score), 0)
		FROM shadow_comparisons;
	`).Scan(&summary.TotalShadowRuns, &summary.AvgShadowAgreement)

	return summary, nil
}

// GetAuditEvents retrieves the chronological log of promotions, rollbacks, and policy events.
func (s *Store) GetAuditEvents(limit int) ([]*AuditEventRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, prompt_id, action, old_version, new_version, reason, triggered_by, metadata_json, created_at
		FROM audit_events
		ORDER BY created_at DESC
		LIMIT ?;
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*AuditEventRecord
	for rows.Next() {
		var e AuditEventRecord
		var createdAtStr string

		err := rows.Scan(
			&e.ID, &e.PromptID, &e.Action, &e.OldVersion, &e.NewVersion,
			&e.Reason, &e.TriggeredBy, &e.MetadataJSON, &createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		events = append(events, &e)
	}

	return events, nil
}
