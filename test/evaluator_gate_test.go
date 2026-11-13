package test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/evaluator"
	"github.com/alexandrmotologa/promptledger/internal/gate"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/store"
)

func TestEvaluator_CompareOutputs(t *testing.T) {
	activeJSON := `{"intent": "PURCHASE", "confidence": 0.95, "action": "ROUTE"}`
	shadowJSON := `{"intent": "PURCHASE", "confidence": 0.98, "action": "ROUTE", "sentiment": "POSITIVE"}`

	quality, sim, verdict, expl := evaluator.CompareOutputs(activeJSON, shadowJSON, 180, 110, 0.0002, 0.00008)

	if quality < 0.85 {
		t.Errorf("expected quality >= 0.85, got %.2f", quality)
	}
	if sim < 0.70 {
		t.Errorf("expected similarity >= 0.70, got %.2f", sim)
	}
	if verdict != "better" {
		t.Errorf("expected verdict 'better', got %s (%s)", verdict, expl)
	}
}

func TestGate_AutomatedPromotionAndRollback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed store: %v", err)
	}
	defer s.Close()

	regDir := filepath.Join(tmpDir, "prompts")
	reg, err := registry.NewRegistry(regDir)
	if err != nil {
		t.Fatalf("failed reg: %v", err)
	}

	spec := &registry.PromptSpec{
		PromptID: "gate-test-prompt",
		Routing: registry.RoutingConfig{
			ActiveVersion: "v1.0.0",
			Variants: []registry.VariantConfig{
				{Version: "v1.0.0", Weight: 80},
				{Version: "v1.1.0-canary", Weight: 20},
			},
		},
		Gates: registry.GateConfig{
			MinQualityScore: 0.85,
			MaxP95LatencyMs: 400,
			MaxErrorRate:    0.02,
			MinSampleSize:   5,
			AutoPromote:     true,
			AutoRollback:    true,
		},
		Versions: map[string]*registry.VersionDetail{
			"v1.0.0": {
				Model:    "gpt-4o-mini",
				Template: "Test {{.UserInput}}",
			},
			"v1.1.0-canary": {
				Model:    "claude-3-5-haiku",
				Template: "Canary {{.UserInput}}",
			},
		},
	}
	_ = reg.SavePrompt(spec)

	gm := gate.NewGateManager(reg, s)

	// Simulate 6 successful runs with high quality
	for i := 0; i < 6; i++ {
		_ = s.RecordExecution(&store.ExecutionRecord{
			ID:          "exec_canary_" + string(rune('a'+i)),
			PromptID:    "gate-test-prompt",
			Version:     "v1.1.0-canary",
			VariantType: "canary",
			LatencyMs:   120,
			CostUSD:     0.00005,
			IsSuccess:   true,
			CreatedAt:   time.Now(),
		})
	}

	// Trigger gate check
	if err := gm.CheckGates("gate-test-prompt"); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	updated, _ := reg.GetPrompt("gate-test-prompt")
	if updated.Routing.ActiveVersion != "v1.1.0-canary" {
		t.Fatalf("expected auto-promotion to v1.1.0-canary, got %s", updated.Routing.ActiveVersion)
	}

	// Now test rollback: simulate error spike on v1.1.0-canary
	// Reset canary to another version
	spec.Routing.ActiveVersion = "v1.0.0"
	spec.Routing.Variants = []registry.VariantConfig{
		{Version: "v1.0.0", Weight: 80},
		{Version: "v1.1.0-canary", Weight: 20},
	}
	_ = reg.SavePrompt(spec)

	// Inject 5 failed executions
	for i := 0; i < 5; i++ {
		_ = s.RecordExecution(&store.ExecutionRecord{
			ID:           "exec_fail_" + string(rune('0'+i)),
			PromptID:     "gate-test-prompt",
			Version:      "v1.1.0-canary",
			VariantType:  "canary",
			LatencyMs:    950,
			CostUSD:      0.00005,
			IsSuccess:    false,
			ErrorMessage: "rate limit exceeded 429",
			CreatedAt:    time.Now(),
		})
	}

	if err := gm.CheckGates("gate-test-prompt"); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	rolledBack, _ := reg.GetPrompt("gate-test-prompt")
	if rolledBack.Routing.ActiveVersion != "v1.0.0" {
		t.Fatalf("expected rollback to v1.0.0, got %s", rolledBack.Routing.ActiveVersion)
	}
}
