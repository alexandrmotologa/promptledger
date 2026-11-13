package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/evaluator"
	"github.com/alexandrmotologa/promptledger/internal/provider"
	"github.com/alexandrmotologa/promptledger/internal/registry"
)

// TestCase represents an individual evaluation line in a JSONL golden dataset.
type TestCase struct {
	ID               string                 `json:"id,omitempty"`
	Input            map[string]interface{} `json:"input"`
	ExpectedIntent   string                 `json:"expected_intent,omitempty"`
	ExpectedContains string                 `json:"expected_contains,omitempty"`
}

// CaseResult holds the individual evaluation outcome.
type CaseResult struct {
	Index        int     `json:"index"`
	Passed       bool    `json:"passed"`
	LatencyMs    int64   `json:"latency_ms"`
	CostUSD      float64 `json:"cost_usd"`
	OutputText   string  `json:"output_text"`
	FailureNotes string  `json:"failure_notes,omitempty"`
}

// EvaluationReport contains aggregated accuracy and latency metrics.
type EvaluationReport struct {
	PromptID        string        `json:"prompt_id"`
	Version         string        `json:"version"`
	Model           string        `json:"model"`
	DatasetPath     string        `json:"dataset_path"`
	TotalCases      int           `json:"total_cases"`
	PassedCases     int           `json:"passed_cases"`
	FailedCases     int           `json:"failed_cases"`
	AccuracyRate    float64       `json:"accuracy_rate"`
	AvgLatencyMs    float64       `json:"avg_latency_ms"`
	P95LatencyMs    int64         `json:"p95_latency_ms"`
	TotalCostUSD    float64       `json:"total_cost_usd"`
	ExecutionTimeMs int64         `json:"execution_time_ms"`
	Results         []CaseResult  `json:"results"`
	MarkdownSummary string        `json:"markdown_summary"`
}

// RunEvaluation executes an offline benchmark against a golden JSONL dataset.
func RunEvaluation(ctx context.Context, spec *registry.PromptSpec, version string, datasetPath string, prov provider.Provider) (*EvaluationReport, error) {
	file, err := os.Open(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dataset file: %w", err)
	}
	defer file.Close()

	vDetail, exists := spec.Versions[version]
	if !exists {
		return nil, fmt.Errorf("version '%s' not found in prompt '%s'", version, spec.PromptID)
	}

	var testCases []TestCase
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var tc TestCase
		if err := json.Unmarshal([]byte(line), &tc); err == nil {
			testCases = append(testCases, tc)
		}
	}

	if len(testCases) == 0 {
		return nil, fmt.Errorf("no test cases found in %s", datasetPath)
	}

	start := time.Now()
	report := &EvaluationReport{
		PromptID:    spec.PromptID,
		Version:     version,
		Model:       vDetail.Model,
		DatasetPath: datasetPath,
		TotalCases:  len(testCases),
	}

	var latencies []int64
	var sumLatency int64

	for idx, tc := range testCases {
		rendered, err := registry.RenderTemplate(vDetail.Template, tc.Input)
		if err != nil {
			report.FailedCases++
			report.Results = append(report.Results, CaseResult{
				Index:        idx + 1,
				Passed:       false,
				FailureNotes: fmt.Sprintf("Template render error: %v", err),
			})
			continue
		}

		res, err := prov.Execute(ctx, vDetail.Model, rendered, nil)
		if err != nil {
			report.FailedCases++
			report.Results = append(report.Results, CaseResult{
				Index:        idx + 1,
				Passed:       false,
				FailureNotes: fmt.Sprintf("Model execution error: %v", err),
			})
			continue
		}

		latencies = append(latencies, res.LatencyMs)
		sumLatency += res.LatencyMs
		report.TotalCostUSD += res.CostUSD

		passed := true
		var failReasons []string

		// Check declarative assertions if present
		if len(spec.Evaluations) > 0 {
			aRes := evaluator.EvaluateAssertions(res.OutputText, spec.Evaluations)
			if !aRes.AllPassed {
				passed = false
				failReasons = append(failReasons, aRes.FailureReasons...)
			}
		}

		// Check expected intent
		if tc.ExpectedIntent != "" {
			if !strings.Contains(strings.ToUpper(res.OutputText), strings.ToUpper(tc.ExpectedIntent)) {
				passed = false
				failReasons = append(failReasons, fmt.Sprintf("Expected intent '%s' missing", tc.ExpectedIntent))
			}
		}

		// Check expected substring
		if tc.ExpectedContains != "" {
			if !strings.Contains(res.OutputText, tc.ExpectedContains) {
				passed = false
				failReasons = append(failReasons, fmt.Sprintf("Expected substring '%s' missing", tc.ExpectedContains))
			}
		}

		if passed {
			report.PassedCases++
		} else {
			report.FailedCases++
		}

		report.Results = append(report.Results, CaseResult{
			Index:        idx + 1,
			Passed:       passed,
			LatencyMs:    res.LatencyMs,
			CostUSD:      res.CostUSD,
			OutputText:   res.OutputText,
			FailureNotes: strings.Join(failReasons, "; "),
		})
	}

	report.ExecutionTimeMs = time.Since(start).Milliseconds()
	if report.TotalCases > 0 {
		report.AccuracyRate = float64(report.PassedCases) / float64(report.TotalCases)
	}

	if len(latencies) > 0 {
		report.AvgLatencyMs = float64(sumLatency) / float64(len(latencies))
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p95Idx := int(float64(len(latencies)) * 0.95)
		if p95Idx >= len(latencies) {
			p95Idx = len(latencies) - 1
		}
		report.P95LatencyMs = latencies[p95Idx]
	}

	// Format Markdown summary table for terminal and CI/CD
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### promptledger Golden Benchmark: `%s` (%s)\n\n", spec.PromptID, version))
	sb.WriteString(fmt.Sprintf("- **Model**: `%s`\n", vDetail.Model))
	sb.WriteString(fmt.Sprintf("- **Pass Rate**: **%.1f%%** (%d/%d test cases passed)\n", report.AccuracyRate*100, report.PassedCases, report.TotalCases))
	sb.WriteString(fmt.Sprintf("- **Avg Latency**: `%.1fms` (P95: `%dms`)\n", report.AvgLatencyMs, report.P95LatencyMs))
	sb.WriteString(fmt.Sprintf("- **Total Cost**: `$%.6f`\n\n", report.TotalCostUSD))
	sb.WriteString("| # | Status | Latency | Cost | Notes |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, r := range report.Results {
		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
		}
		notes := r.FailureNotes
		if notes == "" {
			notes = "Matched all criteria"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %dms | $%.6f | %s |\n", r.Index, status, r.LatencyMs, r.CostUSD, notes))
	}

	report.MarkdownSummary = sb.String()
	return report, nil
}
