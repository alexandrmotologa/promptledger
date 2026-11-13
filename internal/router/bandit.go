package router

import (
	"crypto/rand"
	"math"
	"math/big"
	"sync"

	"github.com/alexandrmotologa/promptledger/internal/registry"
)

// ArmStats tracks cumulative trial counts and average rewards for an individual prompt version.
type ArmStats struct {
	Trials        int     `json:"trials"`
	AvgQuality    float64 `json:"avg_quality"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	AvgCostUSD    float64 `json:"avg_cost_usd"`
	AverageReward float64 `json:"average_reward"` // 0.0 to 1.0
}

// BanditManager maintains in-memory Multi-Armed Bandit state per prompt.
type BanditManager struct {
	mu    sync.RWMutex
	stats map[string]map[string]*ArmStats // prompt_id -> version -> stats
}

// NewBanditManager creates an initialized bandit manager.
func NewBanditManager() *BanditManager {
	return &BanditManager{
		stats: make(map[string]map[string]*ArmStats),
	}
}

// RecordFeedback updates the bandit's knowledge for a given version.
func (bm *BanditManager) RecordFeedback(promptID, version string, quality float64, latencyMs int64, costUSD float64, cfg *registry.BanditConfig) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	pStats, exists := bm.stats[promptID]
	if !exists {
		pStats = make(map[string]*ArmStats)
		bm.stats[promptID] = pStats
	}

	arm, exists := pStats[version]
	if !exists {
		arm = &ArmStats{}
		pStats[version] = arm
	}

	// Calculate normalized reward (0.0 to 1.0)
	qWeight := 0.50
	lWeight := 0.25
	cWeight := 0.25
	if cfg != nil {
		if cfg.QualityWeight > 0 {
			qWeight = cfg.QualityWeight
		}
		if cfg.LatencyWeight > 0 {
			lWeight = cfg.LatencyWeight
		}
		if cfg.CostWeight > 0 {
			cWeight = cfg.CostWeight
		}
	}

	latScore := 1.0 - math.Min(float64(latencyMs)/1000.0, 1.0)
	costScore := 1.0 - math.Min(costUSD/0.005, 1.0)
	if latScore < 0 {
		latScore = 0
	}
	if costScore < 0 {
		costScore = 0
	}

	instantReward := (quality * qWeight) + (latScore * lWeight) + (costScore * cWeight)

	// Online running average update
	arm.Trials++
	n := float64(arm.Trials)
	arm.AverageReward = arm.AverageReward + ((instantReward - arm.AverageReward) / n)
	arm.AvgQuality = arm.AvgQuality + ((quality - arm.AvgQuality) / n)
	arm.AvgLatencyMs = arm.AvgLatencyMs + ((float64(latencyMs) - arm.AvgLatencyMs) / n)
	arm.AvgCostUSD = arm.AvgCostUSD + ((costUSD - arm.AvgCostUSD) / n)
}

// SelectArm chooses the best arm using Epsilon-Greedy exploration/exploitation.
func (bm *BanditManager) SelectArm(promptID string, versions []string, epsilon float64, minTrials int) (string, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if len(versions) == 0 {
		return "", false
	}
	if len(versions) == 1 {
		return versions[0], false
	}

	pStats := bm.stats[promptID]

	// 1. Initial warm-up: ensure each arm has at least minTrials
	if minTrials <= 0 {
		minTrials = 5
	}
	for _, v := range versions {
		if pStats == nil || pStats[v] == nil || pStats[v].Trials < minTrials {
			return v, true // Warm-up exploration
		}
	}

	// 2. Epsilon random exploration
	if epsilon <= 0 {
		epsilon = 0.10
	}
	rVal, _ := rand.Int(rand.Reader, big.NewInt(1000))
	if float64(rVal.Int64())/1000.0 < epsilon {
		// Random choice
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(versions))))
		return versions[idx.Int64()], true // Explored
	}

	// 3. Exploit the arm with the highest average reward
	bestVersion := versions[0]
	bestReward := -1.0

	for _, v := range versions {
		if arm, exists := pStats[v]; exists {
			if arm.AverageReward > bestReward {
				bestReward = arm.AverageReward
				bestVersion = v
			}
		}
	}

	return bestVersion, false // Exploited
}

// GetArmStats returns statistical state of all arms for a prompt.
func (bm *BanditManager) GetArmStats(promptID string) map[string]*ArmStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	res := make(map[string]*ArmStats)
	if pStats, exists := bm.stats[promptID]; exists {
		for k, v := range pStats {
			clone := *v
			res[k] = &clone
		}
	}
	return res
}
