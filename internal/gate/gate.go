package gate

import (
	"fmt"
	"sync"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/store"
)

// GateManager enforces statistical gates for automatic promotion and rollback.
type GateManager struct {
	mu       sync.Mutex
	registry *registry.Registry
	store    *store.Store
}

// NewGateManager creates a new policy gate manager.
func NewGateManager(reg *registry.Registry, s *store.Store) *GateManager {
	return &GateManager{
		registry: reg,
		store:    s,
	}
}

// CheckGates inspects running metrics against defined thresholds for canary or shadow variants.
func (gm *GateManager) CheckGates(promptID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	spec, ok := gm.registry.GetPrompt(promptID)
	if !ok {
		return fmt.Errorf("prompt '%s' not found", promptID)
	}

	metrics, err := gm.store.GetPromptMetrics(promptID)
	if err != nil {
		return err
	}

	activeVer := spec.Routing.ActiveVersion
	activeMetric := metrics.Versions[activeVer]

	// Find active canary version if present
	var canaryVer string
	for _, v := range spec.Routing.Variants {
		if v.Version != activeVer && v.Weight > 0 {
			canaryVer = v.Version
			break
		}
	}

	if canaryVer == "" {
		return nil // No active canary testing currently
	}

	canaryMetric := metrics.Versions[canaryVer]
	if canaryMetric == nil {
		return nil // Not enough data yet
	}

	// 1. Rollback Gate Check (Safety First)
	if spec.Gates.AutoRollback && canaryMetric.TotalRuns >= 5 {
		// If canary error rate exceeds threshold or is dramatically worse than active
		baselineError := 0.0
		if activeMetric != nil {
			baselineError = activeMetric.ErrorRate
		}

		if canaryMetric.ErrorRate > spec.Gates.MaxErrorRate || (canaryMetric.ErrorRate-baselineError) > 0.03 {
			// Trigger automated rollback!
			reason := fmt.Sprintf("Canary error rate (%.1f%%) exceeded ceiling (%.1f%%)", canaryMetric.ErrorRate*100, spec.Gates.MaxErrorRate*100)
			return gm.executeRollback(spec, canaryVer, activeVer, reason, "auto_gate")
		}
	}

	// 2. Promotion Gate Check
	if spec.Gates.AutoPromote && canaryMetric.TotalRuns >= spec.Gates.MinSampleSize {
		passesQuality := spec.Gates.MinQualityScore == 0 ||
			canaryMetric.AvgQualityScore >= spec.Gates.MinQualityScore ||
			metrics.AvgShadowAgreement >= spec.Gates.MinQualityScore ||
			(canaryMetric.ErrorRate == 0 && metrics.AvgShadowAgreement == 0)
		passesLatency := spec.Gates.MaxP95LatencyMs == 0 || canaryMetric.P95LatencyMs <= spec.Gates.MaxP95LatencyMs
		passesError := canaryMetric.ErrorRate <= spec.Gates.MaxErrorRate

		if passesQuality && passesLatency && passesError {
			// Trigger automated promotion to 100%!
			reason := fmt.Sprintf("Canary passed all gates: %d runs, error rate %.1f%%, P95 %dms, quality %.2f",
				canaryMetric.TotalRuns, canaryMetric.ErrorRate*100, canaryMetric.P95LatencyMs, canaryMetric.AvgQualityScore)
			return gm.executePromote(spec, canaryVer, activeVer, reason, "auto_gate")
		}
	}

	return nil
}

// Promote manually or programmatically promotes a version to 100% active traffic.
func (gm *GateManager) Promote(promptID string, targetVersion string, triggeredBy string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	spec, ok := gm.registry.GetPrompt(promptID)
	if !ok {
		return fmt.Errorf("prompt '%s' not found", promptID)
	}

	if _, exists := spec.Versions[targetVersion]; !exists {
		return fmt.Errorf("target version '%s' does not exist in prompt '%s'", targetVersion, promptID)
	}

	oldVer := spec.Routing.ActiveVersion
	reason := fmt.Sprintf("Promoted version '%s' to 100%% active traffic", targetVersion)
	return gm.executePromote(spec, targetVersion, oldVer, reason, triggeredBy)
}

// Rollback restores previous stable version and resets canary weights.
func (gm *GateManager) Rollback(promptID string, triggeredBy string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	spec, ok := gm.registry.GetPrompt(promptID)
	if !ok {
		return fmt.Errorf("prompt '%s' not found", promptID)
	}

	// Determine rollback target: first version in spec that is not current active
	var targetVersion string
	for v := range spec.Versions {
		if v != spec.Routing.ActiveVersion {
			targetVersion = v
			break
		}
	}

	if targetVersion == "" {
		return fmt.Errorf("no alternative version available for rollback")
	}

	oldVer := spec.Routing.ActiveVersion
	reason := fmt.Sprintf("Triggered instant rollback from '%s' to '%s'", oldVer, targetVersion)
	return gm.executeRollback(spec, oldVer, targetVersion, reason, triggeredBy)
}

func (gm *GateManager) executePromote(spec *registry.PromptSpec, targetVer, oldVer, reason, triggeredBy string) error {
	spec.Routing.ActiveVersion = targetVer
	spec.Routing.Variants = []registry.VariantConfig{
		{Version: targetVer, Weight: 100},
	}

	if err := gm.registry.SavePrompt(spec); err != nil {
		return fmt.Errorf("failed saving promoted prompt spec: %w", err)
	}

	_ = gm.store.RecordAuditEvent(&store.AuditEventRecord{
		PromptID:    spec.PromptID,
		Action:      "promote",
		OldVersion:  oldVer,
		NewVersion:  targetVer,
		Reason:      reason,
		TriggeredBy: triggeredBy,
		CreatedAt:   time.Now(),
	})

	return nil
}

func (gm *GateManager) executeRollback(spec *registry.PromptSpec, failedVer, safeVer, reason, triggeredBy string) error {
	spec.Routing.ActiveVersion = safeVer
	spec.Routing.Variants = []registry.VariantConfig{
		{Version: safeVer, Weight: 100},
	}

	if err := gm.registry.SavePrompt(spec); err != nil {
		return fmt.Errorf("failed saving rolled back prompt spec: %w", err)
	}

	_ = gm.store.RecordAuditEvent(&store.AuditEventRecord{
		PromptID:    spec.PromptID,
		Action:      "rollback",
		OldVersion:  failedVer,
		NewVersion:  safeVer,
		Reason:      reason,
		TriggeredBy: triggeredBy,
		CreatedAt:   time.Now(),
	})

	return nil
}
