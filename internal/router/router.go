package router

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math/big"
	"sync"

	"github.com/alexandrmotologa/promptledger/internal/registry"
)

// SelectedVariant represents the chosen version to serve the request.
type SelectedVariant struct {
	PromptID    string
	Version     string
	IsCanary    bool
	Model       string
	Template    string
	Weight      int
	VariantType string // "active", "canary", "shadow"
}

// Router handles sticky and weighted traffic splitting and shadow fork dispatch.
type Router struct {
	mu       sync.RWMutex
	registry *registry.Registry
}

// NewRouter creates a new traffic router backed by the registry.
func NewRouter(reg *registry.Registry) *Router {
	return &Router{
		registry: reg,
	}
}

// Route determines which prompt variant to serve for a given request.
// If userID or tenantID is provided, it uses deterministic sticky hashing.
func (r *Router) Route(promptID string, userID string, tenantID string) (*SelectedVariant, *SelectedVariant, error) {
	spec, ok := r.registry.GetPrompt(promptID)
	if !ok {
		return nil, nil, fmt.Errorf("prompt '%s' not found", promptID)
	}

	if len(spec.Routing.Variants) == 0 {
		return nil, nil, fmt.Errorf("no routing variants configured for '%s'", promptID)
	}

	bucket := r.computeBucket(promptID, userID, tenantID)

	var selected *SelectedVariant
	cumWeight := 0

	for _, v := range spec.Routing.Variants {
		cumWeight += v.Weight
		if bucket < cumWeight {
			vDetail, exists := spec.Versions[v.Version]
			if !exists {
				return nil, nil, fmt.Errorf("version '%s' not found in spec", v.Version)
			}

			isCanary := v.Version != spec.Routing.ActiveVersion
			vType := "active"
			if isCanary {
				vType = "canary"
			}

			selected = &SelectedVariant{
				PromptID:    promptID,
				Version:     v.Version,
				IsCanary:    isCanary,
				Model:       vDetail.Model,
				Template:    vDetail.Template,
				Weight:      v.Weight,
				VariantType: vType,
			}
			break
		}
	}

	// Fallback to active version if boundary missed
	if selected == nil {
		activeVer := spec.Routing.ActiveVersion
		vDetail := spec.Versions[activeVer]
		selected = &SelectedVariant{
			PromptID:    promptID,
			Version:     activeVer,
			IsCanary:    false,
			Model:       vDetail.Model,
			Template:    vDetail.Template,
			Weight:      100,
			VariantType: "active",
		}
	}

	// Determine if shadow fork should be triggered
	var shadowVariant *SelectedVariant
	if spec.Routing.Shadow.Enabled && spec.Routing.Shadow.Version != "" {
		if r.shouldSampleShadow(promptID, userID, tenantID, spec.Routing.Shadow.SampleRate) {
			sDetail, exists := spec.Versions[spec.Routing.Shadow.Version]
			if exists {
				shadowVariant = &SelectedVariant{
					PromptID:    promptID,
					Version:     spec.Routing.Shadow.Version,
					IsCanary:    false,
					Model:       sDetail.Model,
					Template:    sDetail.Template,
					Weight:      0,
					VariantType: "shadow",
				}
			}
		}
	}

	return selected, shadowVariant, nil
}

// computeBucket returns an integer in [0, 99].
func (r *Router) computeBucket(promptID, userID, tenantID string) int {
	seed := userID
	if seed == "" {
		seed = tenantID
	}

	if seed != "" {
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(promptID + ":" + seed))
		return int(hasher.Sum32() % 100)
	}

	// Random distribution for anonymous traffic
	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

// shouldSampleShadow determines whether the request falls into the shadow sample fraction.
func (r *Router) shouldSampleShadow(promptID, userID, tenantID string, sampleRate float64) bool {
	if sampleRate <= 0.0 {
		return false
	}
	if sampleRate >= 1.0 {
		return true
	}

	seed := userID
	if seed == "" {
		seed = tenantID
	}

	if seed != "" {
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte("shadow:" + promptID + ":" + seed))
		pct := float64(hasher.Sum32()%1000) / 1000.0
		return pct < sampleRate
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return false
	}
	return (float64(n.Int64()) / 1000.0) < sampleRate
}
