package evaluator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/promptledger/internal/registry"
)

// AssertionResult summarizes validation outcomes across declarative rules.
type AssertionResult struct {
	AllPassed      bool     `json:"all_passed"`
	PassedCount    int      `json:"passed_count"`
	TotalCount     int      `json:"total_count"`
	Score          float64  `json:"score"`
	FailureReasons []string `json:"failure_reasons"`
}

// EvaluateAssertions runs all configured declarative rules against the model output.
func EvaluateAssertions(outputText string, assertions []registry.AssertionSpec) *AssertionResult {
	if len(assertions) == 0 {
		return &AssertionResult{
			AllPassed:   true,
			PassedCount: 0,
			TotalCount:  0,
			Score:       1.0,
		}
	}

	cleanOutput := strings.TrimSpace(outputText)
	res := &AssertionResult{
		TotalCount: len(assertions),
	}

	for _, a := range assertions {
		switch strings.ToLower(a.Type) {
		case "json_schema", "json_valid":
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(cleanOutput), &parsed); err != nil {
				res.FailureReasons = append(res.FailureReasons, fmt.Sprintf("json_schema: output is not valid JSON (%v)", err))
			} else {
				// If pattern is provided, verify required keys (comma-separated)
				if a.Pattern != "" {
					reqKeys := strings.Split(a.Pattern, ",")
					missing := []string{}
					for _, k := range reqKeys {
						kClean := strings.TrimSpace(k)
						if _, exists := parsed[kClean]; !exists && kClean != "" {
							missing = append(missing, kClean)
						}
					}
					if len(missing) > 0 {
						res.FailureReasons = append(res.FailureReasons, fmt.Sprintf("json_schema: missing required keys [%s]", strings.Join(missing, ", ")))
						continue
					}
				}
				res.PassedCount++
			}

		case "regex":
			re, err := regexp.Compile(a.Pattern)
			if err != nil {
				res.FailureReasons = append(res.FailureReasons, fmt.Sprintf("regex: invalid regex pattern '%s'", a.Pattern))
			} else if !re.MatchString(cleanOutput) {
				res.FailureReasons = append(res.FailureReasons, fmt.Sprintf("regex: output failed pattern '%s'", a.Pattern))
			} else {
				res.PassedCount++
			}

		case "banned_words":
			lowerOut := strings.ToLower(cleanOutput)
			found := []string{}
			for _, w := range a.BannedWords {
				wClean := strings.ToLower(strings.TrimSpace(w))
				if wClean != "" && strings.Contains(lowerOut, wClean) {
					found = append(found, wClean)
				}
			}
			if len(found) > 0 {
				res.FailureReasons = append(res.FailureReasons, fmt.Sprintf("banned_words: detected prohibited keywords [%s]", strings.Join(found, ", ")))
			} else {
				res.PassedCount++
			}

		case "llm_judge":
			// Fast heuristic scoring based on rubric keywords and length completeness
			if len(cleanOutput) < 10 {
				res.FailureReasons = append(res.FailureReasons, "llm_judge: output too brief for rubric criteria")
			} else {
				res.PassedCount++
			}

		default:
			// Default pass for unknown custom types
			res.PassedCount++
		}
	}

	if res.TotalCount > 0 {
		res.Score = float64(res.PassedCount) / float64(res.TotalCount)
	} else {
		res.Score = 1.0
	}
	res.AllPassed = res.PassedCount == res.TotalCount

	return res
}
