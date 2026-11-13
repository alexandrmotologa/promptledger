package test

import (
	"fmt"
	"math"
	"testing"

	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/router"
)

func setupTestRouter() (*router.Router, error) {
	spec := &registry.PromptSpec{
		PromptID: "test-classifier",
		Routing: registry.RoutingConfig{
			ActiveVersion: "v1.0.0",
			Variants: []registry.VariantConfig{
				{Version: "v1.0.0", Weight: 90},
				{Version: "v1.1.0-canary", Weight: 10},
			},
			Shadow: registry.ShadowConfig{
				Enabled:    true,
				Version:    "v1.2.0-shadow",
				SampleRate: 0.10,
			},
		},
		Versions: map[string]*registry.VersionDetail{
			"v1.0.0": {
				Model:    "gpt-4o-mini",
				Template: "Hello {{.UserInput}}",
			},
			"v1.1.0-canary": {
				Model:    "claude-3-5-haiku",
				Template: "Hi {{.UserInput}}",
			},
			"v1.2.0-shadow": {
				Model:    "gemini-2.0-flash",
				Template: "Hey {{.UserInput}}",
			},
		},
	}

	reg, err := registry.NewRegistry("../prompts")
	if err != nil {
		return nil, err
	}
	_ = reg.SavePrompt(spec)

	return router.NewRouter(reg), nil
}

func BenchmarkRouter_Route(b *testing.B) {
	r, err := setupTestRouter()
	if err != nil {
		b.Fatalf("failed setup: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user_%d", i%500)
		_, _, err := r.Route("test-classifier", userID, "")
		if err != nil {
			b.Fatalf("routing error: %v", err)
		}
	}
}

func TestRouter_DeterministicBucketing(t *testing.T) {
	r, err := setupTestRouter()
	if err != nil {
		t.Fatalf("failed setup: %v", err)
	}

	// 100 queries for the exact same user must yield the exact same variant
	userID := "usr_persistent_99182"
	firstVariant, _, err := r.Route("test-classifier", userID, "")
	if err != nil {
		t.Fatalf("route error: %v", err)
	}

	for i := 0; i < 100; i++ {
		v, _, err := r.Route("test-classifier", userID, "")
		if err != nil {
			t.Fatalf("route error: %v", err)
		}
		if v.Version != firstVariant.Version {
			t.Fatalf("inconsistent deterministic routing! expected %s got %s", firstVariant.Version, v.Version)
		}
	}
}

func TestRouter_WeightedDistribution(t *testing.T) {
	r, err := setupTestRouter()
	if err != nil {
		t.Fatalf("failed setup: %v", err)
	}

	total := 10000
	counts := make(map[string]int)

	for i := 0; i < total; i++ {
		userID := fmt.Sprintf("random_user_%d", i)
		v, _, err := r.Route("test-classifier", userID, "")
		if err != nil {
			t.Fatalf("route error: %v", err)
		}
		counts[v.Version]++
	}

	activePct := float64(counts["v1.0.0"]) / float64(total)
	canaryPct := float64(counts["v1.1.0-canary"]) / float64(total)

	// Tolerance of +/- 2.5% on 10,000 requests
	if math.Abs(activePct-0.90) > 0.025 {
		t.Errorf("expected ~90%% for active, got %.2f%%", activePct*100)
	}
	if math.Abs(canaryPct-0.10) > 0.025 {
		t.Errorf("expected ~10%% for canary, got %.2f%%", canaryPct*100)
	}
}
