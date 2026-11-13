package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/provider"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/router"
	"github.com/alexandrmotologa/promptledger/internal/store"
	"github.com/google/uuid"
)

// ShadowTask holds data required for asynchronous background evaluation.
type ShadowTask struct {
	PromptID        string
	ActiveExecution *store.ExecutionRecord
	ShadowVariant   *router.SelectedVariant
	Variables       map[string]interface{}
}

// GateEvaluator provides an interface to notify the automated gate after a shadow run.
type GateEvaluator interface {
	CheckGates(promptID string) error
}

// Evaluator runs a non-blocking background worker pool for shadow execution and scoring.
type Evaluator struct {
	store         *store.Store
	provider      provider.Provider
	tasks         chan *ShadowTask
	workers       int
	stopChan      chan struct{}
	wg            sync.WaitGroup
	gateEvaluator GateEvaluator
}

// NewEvaluator creates and starts the async shadow evaluation pool.
func NewEvaluator(s *store.Store, p provider.Provider, workers int, queueSize int) *Evaluator {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 500
	}

	e := &Evaluator{
		store:    s,
		provider: p,
		tasks:    make(chan *ShadowTask, queueSize),
		workers:  workers,
		stopChan: make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		e.wg.Add(1)
		go e.workerLoop(i)
	}

	return e
}

// SetGateEvaluator sets the callback gate evaluator.
func (e *Evaluator) SetGateEvaluator(ge GateEvaluator) {
	e.gateEvaluator = ge
}

// Submit enqueues a shadow task without blocking the calling goroutine.
func (e *Evaluator) Submit(task *ShadowTask) bool {
	select {
	case e.tasks <- task:
		return true
	default:
		// Drop task if queue is saturated to preserve zero production impact
		return false
	}
}

// Stop gracefully waits for active workers to drain and exit.
func (e *Evaluator) Stop() {
	close(e.stopChan)
	e.wg.Wait()
}

func (e *Evaluator) workerLoop(workerID int) {
	defer e.wg.Done()

	for {
		select {
		case <-e.stopChan:
			return
		case task, ok := <-e.tasks:
			if !ok {
				return
			}
			e.processShadowTask(task)
		}
	}
}

func (e *Evaluator) processShadowTask(task *ShadowTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Render template for shadow variant
	rendered, err := registry.RenderTemplate(task.ShadowVariant.Template, task.Variables)
	if err != nil {
		fmt.Printf("[shadow evaluator error] render failed for %s: %v\n", task.PromptID, err)
		return
	}

	// 2. Execute shadow prompt against provider
	res, err := e.provider.Execute(ctx, task.ShadowVariant.Model, rendered, nil)
	shadowExecID := "shd_" + uuid.New().String()[:12]

	shadowRecord := &store.ExecutionRecord{
		ID:             shadowExecID,
		PromptID:       task.PromptID,
		Version:        task.ShadowVariant.Version,
		VariantType:    "shadow",
		UserID:         task.ActiveExecution.UserID,
		TenantID:       task.ActiveExecution.TenantID,
		InputVariables: task.Variables,
		RenderedPrompt: rendered,
		CreatedAt:      time.Now(),
	}

	if err != nil {
		shadowRecord.IsSuccess = false
		shadowRecord.ErrorMessage = err.Error()
		_ = e.store.RecordExecution(shadowRecord)
		return
	}

	shadowRecord.OutputText = res.OutputText
	shadowRecord.ModelUsed = res.ModelUsed
	shadowRecord.LatencyMs = res.LatencyMs
	shadowRecord.PromptTokens = res.PromptTokens
	shadowRecord.CompletionTokens = res.CompletionTokens
	shadowRecord.TotalTokens = res.TotalTokens
	shadowRecord.CostUSD = res.CostUSD
	shadowRecord.IsSuccess = true

	if err := e.store.RecordExecution(shadowRecord); err != nil {
		fmt.Printf("[shadow evaluator error] failed saving execution: %v\n", err)
	}

	// 3. Compute semantic similarity and quality scores
	qualityScore, similarity, verdict, explanation := CompareOutputs(
		task.ActiveExecution.OutputText,
		res.OutputText,
		task.ActiveExecution.LatencyMs,
		res.LatencyMs,
		task.ActiveExecution.CostUSD,
		res.CostUSD,
	)

	latencyDelta := res.LatencyMs - task.ActiveExecution.LatencyMs
	costDelta := res.CostUSD - task.ActiveExecution.CostUSD

	comparison := &store.ShadowComparisonRecord{
		PromptID:           task.PromptID,
		ActiveExecutionID:  task.ActiveExecution.ID,
		ShadowExecutionID:  shadowExecID,
		ActiveVersion:      task.ActiveExecution.Version,
		ShadowVersion:      task.ShadowVariant.Version,
		QualityScore:       qualityScore,
		LatencyDeltaMs:     latencyDelta,
		CostDeltaUSD:       costDelta,
		SemanticSimilarity: similarity,
		Verdict:            verdict,
		Explanation:        explanation,
		CreatedAt:          time.Now(),
	}

	if err := e.store.RecordShadowComparison(comparison); err != nil {
		fmt.Printf("[shadow evaluator error] failed saving comparison: %v\n", err)
	}

	// 4. Trigger safety gate evaluation if configured
	if e.gateEvaluator != nil {
		_ = e.gateEvaluator.CheckGates(task.PromptID)
	}
}

// CompareOutputs evaluates similarity, quality, latency, and cost deltas.
func CompareOutputs(activeText, shadowText string, activeLat, shadowLat int64, activeCost, shadowCost float64) (qualityScore float64, similarity float64, verdict string, explanation string) {
	actClean := strings.TrimSpace(activeText)
	shdClean := strings.TrimSpace(shadowText)

	// Check JSON validity and intent match if both are JSON
	var actJSON, shdJSON map[string]interface{}
	actIsJSON := json.Unmarshal([]byte(actClean), &actJSON) == nil
	shdIsJSON := json.Unmarshal([]byte(shdClean), &shdJSON) == nil

	if actIsJSON && shdIsJSON {
		// Calculate JSON key/value agreement
		matchingKeys := 0
		totalKeys := len(actJSON)
		if totalKeys == 0 {
			totalKeys = 1
		}

		intentMatch := false
		if actVal, ok := actJSON["intent"]; ok {
			if shdVal, ok2 := shdJSON["intent"]; ok2 && actVal == shdVal {
				intentMatch = true
			}
		} else if actVal, ok := actJSON["target_tier"]; ok {
			if shdVal, ok2 := shdJSON["target_tier"]; ok2 && actVal == shdVal {
				intentMatch = true
			}
		}

		for k, v := range actJSON {
			if shdVal, exists := shdJSON[k]; exists {
				if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", shdVal) {
					matchingKeys++
				} else {
					// Numeric tolerance check (e.g. 0.95 vs 0.98)
					vF, ok1 := v.(float64)
					sF, ok2 := shdVal.(float64)
					if ok1 && ok2 && math.Abs(vF-sF) <= 0.05 {
						matchingKeys++
					}
				}
			}
		}

		similarity = float64(matchingKeys) / float64(totalKeys)
		if similarity > 1.0 {
			similarity = 1.0
		}

		qualityScore = 0.5 + (0.3 * similarity)
		if intentMatch {
			qualityScore += 0.2
		}
	} else {
		// Text token Jaccard similarity fallback
		actTokens := tokenize(actClean)
		shdTokens := tokenize(shdClean)
		similarity = jaccardSimilarity(actTokens, shdTokens)
		qualityScore = math.Max(0.60, similarity)
	}

	// Normalize quality score between 0.0 and 1.0
	if qualityScore > 1.0 {
		qualityScore = 1.0
	}
	if qualityScore < 0.0 {
		qualityScore = 0.0
	}

	// Verdict logic
	latImprovement := activeLat - shadowLat // positive means shadow is faster
	costImprovement := activeCost - shadowCost // positive means shadow is cheaper

	if qualityScore >= 0.88 && (latImprovement > 0 || costImprovement > 0) {
		verdict = "better"
		explanation = fmt.Sprintf("High quality agreement (%.0f%%) with %dms latency improvement and lower cost", qualityScore*100, latImprovement)
	} else if qualityScore >= 0.82 {
		verdict = "equivalent"
		explanation = fmt.Sprintf("Parity match (%.0f%% quality) within standard variance bounds", qualityScore*100)
	} else {
		verdict = "regressed"
		explanation = fmt.Sprintf("Output deviated from baseline quality (score: %.2f)", qualityScore)
	}

	return qualityScore, similarity, verdict, explanation
}

func tokenize(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	counts := make(map[string]int)
	for _, w := range words {
		clean := strings.Trim(w, `",.:;{}[]()!?'`)
		if len(clean) > 0 {
			counts[clean]++
		}
	}
	return counts
}

func jaccardSimilarity(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}
