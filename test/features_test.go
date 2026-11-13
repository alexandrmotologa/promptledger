package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/cache"
	"github.com/alexandrmotologa/promptledger/internal/eval"
	"github.com/alexandrmotologa/promptledger/internal/evaluator"
	"github.com/alexandrmotologa/promptledger/internal/metrics"
	"github.com/alexandrmotologa/promptledger/internal/provider"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/router"
	"github.com/alexandrmotologa/promptledger/internal/store"
)

func TestCache_HitMissAndFinancials(t *testing.T) {
	c := cache.NewCache()

	key := cache.GenerateKey("intent", "v1.0.0", map[string]interface{}{"UserInput": "Buy shoes"})
	
	// 1. Initial lookup -> Miss
	_, hit := c.Get(key)
	if hit {
		t.Fatalf("expected cache miss")
	}

	// 2. Set entry
	resp := &provider.Response{
		OutputText: "{\"intent\": \"PURCHASE\"}",
		LatencyMs:  120,
		CostUSD:    0.00015,
	}
	c.Set(key, resp, 10*time.Minute)

	// 3. Second lookup -> Hit
	cached, hit := c.Get(key)
	if !hit {
		t.Fatalf("expected cache hit")
	}
	if cached.LatencyMs != 1 {
		t.Errorf("expected 1ms latency for cached response, got %d", cached.LatencyMs)
	}

	stats := c.GetStats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("expected 1 hit and 1 miss, got %d hits and %d misses", stats.Hits, stats.Misses)
	}
	if stats.TotalDollarsSaved < 0.00014 {
		t.Errorf("expected dollars saved, got %f", stats.TotalDollarsSaved)
	}
}

func TestEvaluator_CustomAssertions(t *testing.T) {
	assertions := []registry.AssertionSpec{
		{Type: "json_schema", Pattern: "intent,confidence"},
		{Type: "banned_words", BannedWords: []string{"hallucination", "competitor_x"}},
		{Type: "regex", Pattern: `^\{.*\}$`},
	}

	// Good output
	goodOutput := `{"intent": "PURCHASE", "confidence": 0.98}`
	resGood := evaluator.EvaluateAssertions(goodOutput, assertions)
	if !resGood.AllPassed || resGood.Score < 1.0 {
		t.Errorf("expected good output to pass all assertions, got %v (%s)", resGood.AllPassed, strings.Join(resGood.FailureReasons, "; "))
	}

	// Bad output: missing key, has banned word
	badOutput := `{"intent": "PURCHASE", "note": "Contains competitor_x"}`
	resBad := evaluator.EvaluateAssertions(badOutput, assertions)
	if resBad.AllPassed {
		t.Errorf("expected bad output to fail assertions")
	}
	if len(resBad.FailureReasons) < 2 {
		t.Errorf("expected at least 2 failure reasons, got %d", len(resBad.FailureReasons))
	}
}

func TestRouter_MultiArmedBandit(t *testing.T) {
	bm := router.NewBanditManager()
	arms := []string{"v1.0.0", "v1.1.0-canary"}

	// Warmup trials
	for i := 0; i < 5; i++ {
		bm.RecordFeedback("prompt-bandit", "v1.0.0", 0.70, 200, 0.001, nil)
		bm.RecordFeedback("prompt-bandit", "v1.1.0-canary", 0.98, 90, 0.0002, nil)
	}

	stats := bm.GetArmStats("prompt-bandit")
	if stats["v1.1.0-canary"].AverageReward <= stats["v1.0.0"].AverageReward {
		t.Fatalf("expected v1.1.0-canary to have higher reward")
	}

	// With epsilon = 0 (pure exploitation), it must pick the highest reward arm
	chosen, isExplored := bm.SelectArm("prompt-bandit", arms, 0.0, 5)
	if chosen != "v1.1.0-canary" {
		t.Errorf("expected exploited arm to be v1.1.0-canary, got %s", chosen)
	}
	if isExplored {
		t.Errorf("expected isExplored=false for pure exploitation")
	}
}

func TestMetrics_PrometheusExporter(t *testing.T) {
	reg, _ := registry.NewRegistry("../prompts")
	s, _ := store.NewStore(":memory:")
	defer s.Close()
	c := cache.NewCache()

	exp := metrics.NewExporter(reg, s, c)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	exp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "promptledger_invocations_total") {
		t.Errorf("expected prometheus invocations metric")
	}
	if !strings.Contains(body, "promptledger_cache_hit_ratio") {
		t.Errorf("expected cache hit ratio metric")
	}
}

func TestEval_GoldenDataset(t *testing.T) {
	spec, err := registry.LoadPromptFile("../prompts/checkout-intent-classifier.yaml")
	if err != nil {
		t.Fatalf("failed loading spec: %v", err)
	}

	prov := provider.NewClient()
	report, err := eval.RunEvaluation(context.Background(), spec, "v1.2.0", "../evals/checkout_intent_eval.jsonl", prov)
	if err != nil {
		t.Fatalf("failed eval run: %v", err)
	}

	if report.TotalCases != 5 {
		t.Errorf("expected 5 test cases, got %d", report.TotalCases)
	}
	if report.AccuracyRate < 0.80 {
		t.Errorf("expected accuracy rate >= 80%%, got %.1f%%", report.AccuracyRate*100)
	}
	if len(report.MarkdownSummary) == 0 {
		t.Errorf("expected markdown summary")
	}
}
