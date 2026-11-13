package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/cache"
	"github.com/alexandrmotologa/promptledger/internal/eval"
	"github.com/alexandrmotologa/promptledger/internal/evaluator"
	"github.com/alexandrmotologa/promptledger/internal/gate"
	"github.com/alexandrmotologa/promptledger/internal/metrics"
	"github.com/alexandrmotologa/promptledger/internal/provider"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/router"
	"github.com/alexandrmotologa/promptledger/internal/store"
	"github.com/google/uuid"
)

// Server coordinates routing, execution, analytics, and studio delivery.
type Server struct {
	registry        *registry.Registry
	router          *router.Router
	provider        provider.Provider
	store           *store.Store
	evaluator       *evaluator.Evaluator
	gateManager     *gate.GateManager
	cache           *cache.Cache
	metricsExporter *metrics.Exporter
	staticFS        fs.FS
	mux             *http.ServeMux
}

// NewServer builds an API server instance.
func NewServer(
	reg *registry.Registry,
	r *router.Router,
	p provider.Provider,
	s *store.Store,
	evalPool *evaluator.Evaluator,
	gm *gate.GateManager,
	c *cache.Cache,
	staticFS fs.FS,
) *Server {
	if c == nil {
		c = cache.NewCache()
	}

	exporter := metrics.NewExporter(reg, s, c)

	srv := &Server{
		registry:        reg,
		router:          r,
		provider:        p,
		store:           s,
		evaluator:       evalPool,
		gateManager:     gm,
		cache:           c,
		metricsExporter: exporter,
		staticFS:        staticFS,
		mux:             http.NewServeMux(),
	}

	evalPool.SetGateEvaluator(gm)
	srv.registerRoutes()
	return srv
}

func (s *Server) registerRoutes() {
	// Prometheus metrics endpoint
	s.mux.Handle("/metrics", s.metricsExporter)

	// API routes
	s.mux.HandleFunc("/v1/prompts", s.handlePrompts)
	s.mux.HandleFunc("/v1/prompts/", s.handlePromptSubroutes)
	s.mux.HandleFunc("/v1/analytics/summary", s.handleAnalyticsSummary)
	s.mux.HandleFunc("/v1/audit-logs", s.handleAuditLogs)
	s.mux.HandleFunc("/v1/simulate-traffic", s.handleSimulateTraffic)
	s.mux.HandleFunc("/v1/cache/stats", s.handleCacheStats)
	s.mux.HandleFunc("/v1/cache/clear", s.handleCacheClear)

	// Static web studio
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/metrics" {
				http.NotFound(w, r)
				return
			}

			// Check if file exists in staticFS
			f, err := s.staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}

			// SPA Fallback to index.html
			indexFile, err := s.staticFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()

			content, _ := io.ReadAll(indexFile)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
		})
	}
}

// ServeHTTP implements http.Handler with CORS support.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompts := s.registry.ListPrompts()
	writeJSON(w, http.StatusOK, prompts)
}

func (s *Server) handlePromptSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/prompts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	promptID := parts[0]

	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			spec, ok := s.registry.GetPrompt(promptID)
			if !ok {
				http.Error(w, "Prompt not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, spec)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := parts[1]

	switch action {
	case "execute":
		s.handleExecute(w, r, promptID)
	case "promote":
		s.handlePromote(w, r, promptID)
	case "rollback":
		s.handleRollback(w, r, promptID)
	case "routing":
		s.handleUpdateRouting(w, r, promptID)
	case "metrics":
		s.handlePromptMetrics(w, r, promptID)
	case "runs":
		s.handlePromptRuns(w, r, promptID)
	case "shadow-diff":
		s.handleShadowDiff(w, r, promptID)
	case "bandit":
		s.handleBanditStats(w, r, promptID)
	case "eval":
		s.handleDatasetEval(w, r, promptID)
	case "test":
		s.handleTest(w, r, promptID)
	default:
		http.NotFound(w, r)
	}
}

type ExecuteRequest struct {
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	Variables map[string]interface{} `json:"variables"`
}

type WaterfallTiming struct {
	RoutingOverheadUs int64 `json:"routing_overhead_us"`
	ProviderLatencyMs int64 `json:"provider_latency_ms"`
	TotalLatencyMs    int64 `json:"total_latency_ms"`
}

type ExecuteResponse struct {
	ID               string           `json:"id"`
	PromptID         string           `json:"prompt_id"`
	Version          string           `json:"version"`
	VariantType      string           `json:"variant_type"`
	OutputText       string           `json:"output_text"`
	ModelUsed        string           `json:"model_used"`
	LatencyMs        int64            `json:"latency_ms"`
	PromptTokens     int              `json:"prompt_tokens"`
	CompletionTokens int              `json:"completion_tokens"`
	TotalTokens      int              `json:"total_tokens"`
	CostUSD          float64          `json:"cost_usd"`
	FromCache        bool             `json:"from_cache"`
	BudgetDownscaled bool             `json:"budget_downscaled"`
	ShadowDispatched bool             `json:"shadow_dispatched"`
	AssertionsPassed bool             `json:"assertions_passed"`
	Timing           WaterfallTiming  `json:"timing"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	startTotal := time.Now()

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	spec, ok := s.registry.GetPrompt(promptID)
	if !ok {
		http.Error(w, fmt.Sprintf("prompt '%s' not found", promptID), http.StatusNotFound)
		return
	}

	// 1. Measure and route request
	routeStart := time.Now()
	selected, shadow, err := s.router.Route(promptID, req.UserID, req.TenantID)
	routingOverheadUs := time.Since(routeStart).Microseconds()
	if err != nil {
		http.Error(w, "Routing error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Check semantic cache if enabled
	cacheKey := ""
	if spec.Cache.Enabled && s.cache != nil {
		cacheKey = cache.GenerateKey(promptID, selected.Version, req.Variables)
		if cachedResp, hit := s.cache.Get(cacheKey); hit {
			totalMs := time.Since(startTotal).Milliseconds()
			if totalMs == 0 {
				totalMs = 1
			}

			resp := ExecuteResponse{
				ID:               "cache_" + uuid.New().String()[:12],
				PromptID:         promptID,
				Version:          selected.Version,
				VariantType:      selected.VariantType,
				OutputText:       cachedResp.OutputText,
				ModelUsed:        cachedResp.ModelUsed,
				LatencyMs:        1,
				PromptTokens:     cachedResp.PromptTokens,
				CompletionTokens: cachedResp.CompletionTokens,
				TotalTokens:      cachedResp.TotalTokens,
				CostUSD:          0.0, // 100% saved
				FromCache:        true,
				ShadowDispatched: false,
				AssertionsPassed: true,
				Timing: WaterfallTiming{
					RoutingOverheadUs: routingOverheadUs,
					ProviderLatencyMs: 0,
					TotalLatencyMs:    totalMs,
				},
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	// 3. Check budget guardrails & model downscale
	modelTarget := selected.Model
	budgetDownscaled := false
	if spec.Budget.MaxDailyCostUSD > 0 && spec.Budget.FallbackModel != "" {
		m, _ := s.store.GetPromptMetrics(promptID)
		dailySpend := 0.0
		if m != nil {
			for _, vm := range m.Versions {
				dailySpend += vm.TotalCostUSD
			}
		}
		if dailySpend >= spec.Budget.MaxDailyCostUSD {
			modelTarget = spec.Budget.FallbackModel
			budgetDownscaled = true
		}
	}

	// 4. Render prompt template
	rendered, err := registry.RenderTemplate(selected.Template, req.Variables)
	if err != nil {
		http.Error(w, "Template render error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 5. Execute prompt with fallback circuit breaker
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	provStart := time.Now()
	res, err := s.provider.Execute(ctx, modelTarget, rendered, nil)
	provLatencyMs := time.Since(provStart).Milliseconds()

	// Fallback chain trigger if model invocation failed
	if err != nil && selected.FallbackVersion != "" && selected.FallbackVersion != selected.Version {
		if fbDetail, exists := spec.Versions[selected.FallbackVersion]; exists {
			fbRendered, fbErr := registry.RenderTemplate(fbDetail.Template, req.Variables)
			if fbErr == nil {
				res, err = s.provider.Execute(ctx, fbDetail.Model, fbRendered, nil)
				if err == nil {
					selected.Version = selected.FallbackVersion
					selected.Model = fbDetail.Model
					rendered = fbRendered
				}
			}
		}
	}

	if err != nil {
		http.Error(w, "Execution error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// 6. Check custom assertions if configured
	assertionsPassed := true
	if len(spec.Evaluations) > 0 {
		aRes := evaluator.EvaluateAssertions(res.OutputText, spec.Evaluations)
		assertionsPassed = aRes.AllPassed
	}

	// 7. Store in semantic cache if enabled
	if spec.Cache.Enabled && s.cache != nil && cacheKey != "" {
		ttl := time.Duration(spec.Cache.TTLSeconds) * time.Second
		s.cache.Set(cacheKey, res, ttl)
	}

	// 8. Record bandit feedback if applicable
	quality := 0.95
	if !assertionsPassed {
		quality = 0.50
	}
	s.router.GetBanditManager().RecordFeedback(promptID, selected.Version, quality, res.LatencyMs, res.CostUSD, &spec.Routing.Bandit)

	execID := "exec_" + uuid.New().String()[:12]
	activeRecord := &store.ExecutionRecord{
		ID:               execID,
		PromptID:         promptID,
		Version:          selected.Version,
		VariantType:      selected.VariantType,
		UserID:           req.UserID,
		TenantID:         req.TenantID,
		InputVariables:   req.Variables,
		RenderedPrompt:   rendered,
		OutputText:       res.OutputText,
		ModelUsed:        res.ModelUsed,
		LatencyMs:        res.LatencyMs,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		TotalTokens:      res.TotalTokens,
		CostUSD:          res.CostUSD,
		IsSuccess:        true,
		CreatedAt:        time.Now(),
	}

	_ = s.store.RecordExecution(activeRecord)

	// 9. Non-blocking shadow dispatch if sampled
	shadowDispatched := false
	if shadow != nil {
		shadowDispatched = s.evaluator.Submit(&evaluator.ShadowTask{
			PromptID:        promptID,
			ActiveExecution: activeRecord,
			ShadowVariant:   shadow,
			Variables:       req.Variables,
		})
	}

	totalMs := time.Since(startTotal).Milliseconds()

	resp := ExecuteResponse{
		ID:               execID,
		PromptID:         promptID,
		Version:          selected.Version,
		VariantType:      selected.VariantType,
		OutputText:       res.OutputText,
		ModelUsed:        res.ModelUsed,
		LatencyMs:        res.LatencyMs,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		TotalTokens:      res.TotalTokens,
		CostUSD:          res.CostUSD,
		FromCache:        false,
		BudgetDownscaled: budgetDownscaled,
		ShadowDispatched: shadowDispatched,
		AssertionsPassed: assertionsPassed,
		Timing: WaterfallTiming{
			RoutingOverheadUs: routingOverheadUs,
			ProviderLatencyMs: provLatencyMs,
			TotalLatencyMs:    totalMs,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePromote(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version string `json:"version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	spec, ok := s.registry.GetPrompt(promptID)
	if !ok {
		http.Error(w, "Prompt not found", http.StatusNotFound)
		return
	}

	targetVer := req.Version
	if targetVer == "" {
		for _, v := range spec.Routing.Variants {
			if v.Version != spec.Routing.ActiveVersion {
				targetVer = v.Version
				break
			}
		}
	}

	if targetVer == "" {
		http.Error(w, "Target version required for promotion", http.StatusBadRequest)
		return
	}

	if err := s.gateManager.Promote(promptID, targetVer, "studio_user"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"promoted_version": targetVer,
		"prompt_id":        promptID,
	})
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.gateManager.Rollback(promptID, "studio_user"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	spec, _ := s.registry.GetPrompt(promptID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"active_version": spec.Routing.ActiveVersion,
		"prompt_id":      promptID,
	})
}

func (s *Server) handleUpdateRouting(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registry.RoutingConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	spec, ok := s.registry.GetPrompt(promptID)
	if !ok {
		http.Error(w, "Prompt not found", http.StatusNotFound)
		return
	}

	spec.Routing = req
	if err := s.registry.SavePrompt(spec); err != nil {
		http.Error(w, "Failed saving routing: "+err.Error(), http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAuditEvent(&store.AuditEventRecord{
		PromptID:    promptID,
		Action:      "routing_update",
		Reason:      fmt.Sprintf("Traffic routing adjustment (Strategy: %s)", req.Strategy),
		TriggeredBy: "studio_user",
		CreatedAt:   time.Now(),
	})

	writeJSON(w, http.StatusOK, spec)
}

func (s *Server) handlePromptMetrics(w http.ResponseWriter, r *http.Request, promptID string) {
	metrics, err := s.store.GetPromptMetrics(promptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	spec, ok := s.registry.GetPrompt(promptID)
	if ok {
		metrics.ActiveVersion = spec.Routing.ActiveVersion
		for _, v := range spec.Routing.Variants {
			if v.Version != spec.Routing.ActiveVersion && v.Weight > 0 {
				vm := metrics.Versions[v.Version]
				if vm != nil && vm.TotalRuns >= spec.Gates.MinSampleSize && vm.ErrorRate <= spec.Gates.MaxErrorRate {
					metrics.AutoPromoteCandidate = true
				}
				if vm != nil && vm.ErrorRate > spec.Gates.MaxErrorRate {
					metrics.RollbackWarning = true
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handlePromptRuns(w http.ResponseWriter, r *http.Request, promptID string) {
	runs, err := s.store.GetRecentExecutions(promptID, 40)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleShadowDiff(w http.ResponseWriter, r *http.Request, promptID string) {
	diffs, err := s.store.GetRecentShadowComparisons(promptID, 40)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, diffs)
}

func (s *Server) handleBanditStats(w http.ResponseWriter, r *http.Request, promptID string) {
	stats := s.router.GetBanditManager().GetArmStats(promptID)
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleDatasetEval(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version string `json:"version"`
		Dataset string `json:"dataset"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	spec, ok := s.registry.GetPrompt(promptID)
	if !ok {
		http.Error(w, "Prompt not found", http.StatusNotFound)
		return
	}

	version := req.Version
	if version == "" {
		version = spec.Routing.ActiveVersion
	}

	datasetPath := req.Dataset
	if datasetPath == "" {
		datasetPath = "evals/checkout_intent_eval.jsonl"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	report, err := eval.RunEvaluation(ctx, spec, version, datasetPath, s.provider)
	if err != nil {
		http.Error(w, "Benchmark execution failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	stats := s.cache.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.cache.Clear()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Semantic cache flushed successfully",
	})
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request, promptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version   string                 `json:"version"`
		Variables map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	spec, ok := s.registry.GetPrompt(promptID)
	if !ok {
		http.Error(w, "Prompt not found", http.StatusNotFound)
		return
	}

	ver := req.Version
	if ver == "" {
		ver = spec.Routing.ActiveVersion
	}

	vDetail, exists := spec.Versions[ver]
	if !exists {
		http.Error(w, "Version not found in prompt", http.StatusBadRequest)
		return
	}

	rendered, err := registry.RenderTemplate(vDetail.Template, req.Variables)
	if err != nil {
		http.Error(w, "Template render error: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	start := time.Now()
	res, err := s.provider.Execute(ctx, vDetail.Model, rendered, nil)
	if err != nil {
		http.Error(w, "Execution error: "+err.Error(), http.StatusBadGateway)
		return
	}

	timingMs := time.Since(start).Milliseconds()

	assertionsResult := evaluator.EvaluateAssertions(res.OutputText, spec.Evaluations)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":           ver,
		"model":             vDetail.Model,
		"rendered_template": rendered,
		"response":          res,
		"timing_ms":         timingMs,
		"assertions":        assertionsResult,
	})
}

func (s *Server) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.GetSystemSummary()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	canaryCount := 0
	for _, p := range s.registry.ListPrompts() {
		for _, v := range p.Routing.Variants {
			if v.Version != p.Routing.ActiveVersion && v.Weight > 0 {
				canaryCount++
				break
			}
		}
	}
	summary.ActiveCanaries = canaryCount

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.GetAuditEvents(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleSimulateTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Count <= 0 {
		req.Count = 10
	}
	if req.Count > 100 {
		req.Count = 100
	}

	sampleInputs := []struct {
		promptID string
		vars     map[string]interface{}
	}{
		{
			promptID: "checkout-intent-classifier",
			vars: map[string]interface{}{
				"UserInput": "I want to purchase the annual plan right now, how do I apply my discount code?",
				"Context":   "Returning customer, cart items: 1",
			},
		},
		{
			promptID: "checkout-intent-classifier",
			vars: map[string]interface{}{
				"UserInput": "My order arrived broken and I want an immediate refund to my card.",
				"Context":   "Delivered 2 days ago, order #8841",
			},
		},
		{
			promptID: "checkout-intent-classifier",
			vars: map[string]interface{}{
				"UserInput": "Do you offer international shipping to Germany and how long does standard transit take?",
				"Context":   "Guest browsing product catalog",
			},
		},
		{
			promptID: "customer-support-rag-router",
			vars: map[string]interface{}{
				"IssueDescription": "Postgres connection pool exhausted with FATAL: remaining connection slots reserved for non-replication superuser connections",
				"CustomerTier":     "ENTERPRISE_PLATINUM",
				"TelemetryState":   "CPU: 89%, Memory: 92%, Active Connections: 500/500",
			},
		},
		{
			promptID: "customer-support-rag-router",
			vars: map[string]interface{}{
				"IssueDescription": "Where can I download the VAT invoice for our company's October billing cycle?",
				"CustomerTier":     "PRO_MONTHLY",
				"TelemetryState":   "Account standing: Good",
			},
		},
		{
			promptID: "sql-generation-agent",
			vars: map[string]interface{}{
				"DatabaseSchema": "users(id, email, role, created_at), subscriptions(id, user_id, status, amount_cents)",
				"UserQuestion":   "Find total revenue from active subscriptions created in the last 30 days grouped by user role",
			},
		},
	}

	executed := 0
	ctx := context.Background()

	for i := 0; i < req.Count; i++ {
		sample := sampleInputs[i%len(sampleInputs)]
		userID := fmt.Sprintf("usr_%d", 1000+(i%15))

		selected, shadow, err := s.router.Route(sample.promptID, userID, "")
		if err != nil {
			continue
		}

		rendered, err := registry.RenderTemplate(selected.Template, sample.vars)
		if err != nil {
			continue
		}

		res, err := s.provider.Execute(ctx, selected.Model, rendered, nil)
		if err != nil {
			continue
		}

		execID := fmt.Sprintf("sim_%d_%s", time.Now().UnixNano(), uuid.New().String()[:6])
		rec := &store.ExecutionRecord{
			ID:               execID,
			PromptID:         sample.promptID,
			Version:          selected.Version,
			VariantType:      selected.VariantType,
			UserID:           userID,
			InputVariables:   sample.vars,
			RenderedPrompt:   rendered,
			OutputText:       res.OutputText,
			ModelUsed:        res.ModelUsed,
			LatencyMs:        res.LatencyMs,
			PromptTokens:     res.PromptTokens,
			CompletionTokens: res.CompletionTokens,
			TotalTokens:      res.TotalTokens,
			CostUSD:          res.CostUSD,
			IsSuccess:        true,
			CreatedAt:        time.Now().Add(-time.Duration((req.Count-i)*2) * time.Minute),
		}

		_ = s.store.RecordExecution(rec)

		// Record bandit feedback
		s.router.GetBanditManager().RecordFeedback(sample.promptID, selected.Version, 0.94, res.LatencyMs, res.CostUSD, nil)

		if shadow != nil {
			s.evaluator.Submit(&evaluator.ShadowTask{
				PromptID:        sample.promptID,
				ActiveExecution: rec,
				ShadowVariant:   shadow,
				Variables:       sample.vars,
			})
		}
		executed++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"simulated_count": executed,
		"status":          "traffic simulation completed",
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
