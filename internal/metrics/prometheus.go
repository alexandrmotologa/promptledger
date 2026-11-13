package metrics

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/promptledger/internal/cache"
	"github.com/alexandrmotologa/promptledger/internal/registry"
	"github.com/alexandrmotologa/promptledger/internal/store"
)

// Exporter formats runtime stats into Prometheus metrics text representation.
type Exporter struct {
	registry *registry.Registry
	store    *store.Store
	cache    *cache.Cache
}

// NewExporter creates a Prometheus metrics exporter.
func NewExporter(reg *registry.Registry, s *store.Store, c *cache.Cache) *Exporter {
	return &Exporter{
		registry: reg,
		store:    s,
		cache:    c,
	}
}

// ServeHTTP writes standard Prometheus metrics format.
func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var sb strings.Builder

	// Header comments
	sb.WriteString("# HELP promptledger_invocations_total Total executions by prompt and version\n")
	sb.WriteString("# TYPE promptledger_invocations_total counter\n")

	sb.WriteString("# HELP promptledger_latency_ms Average latency in milliseconds\n")
	sb.WriteString("# TYPE promptledger_latency_ms gauge\n")

	sb.WriteString("# HELP promptledger_cost_usd Total estimated model spend in USD\n")
	sb.WriteString("# TYPE promptledger_cost_usd counter\n")

	sb.WriteString("# HELP promptledger_shadow_agreement_ratio Quality agreement ratio between active and shadow variants\n")
	sb.WriteString("# TYPE promptledger_shadow_agreement_ratio gauge\n")

	for _, p := range e.registry.ListPrompts() {
		m, err := e.store.GetPromptMetrics(p.PromptID)
		if err != nil {
			continue
		}

		for vName, vMetric := range m.Versions {
			status := "success"
			if vMetric.ErrorCount > 0 && vMetric.TotalRuns == vMetric.ErrorCount {
				status = "error"
			}

			sb.WriteString(fmt.Sprintf(
				"promptledger_invocations_total{prompt=\"%s\",version=\"%s\",variant=\"%s\",status=\"%s\"} %d\n",
				p.PromptID, vName, vMetric.VariantType, status, vMetric.TotalRuns,
			))

			sb.WriteString(fmt.Sprintf(
				"promptledger_latency_ms{prompt=\"%s\",version=\"%s\"} %.2f\n",
				p.PromptID, vName, vMetric.AvgLatencyMs,
			))

			sb.WriteString(fmt.Sprintf(
				"promptledger_cost_usd{prompt=\"%s\",version=\"%s\"} %.6f\n",
				p.PromptID, vName, vMetric.TotalCostUSD,
			))
		}

		if m.ShadowComparisons > 0 {
			sb.WriteString(fmt.Sprintf(
				"promptledger_shadow_agreement_ratio{prompt=\"%s\"} %.4f\n",
				p.PromptID, m.AvgShadowAgreement,
			))
		}
	}

	// Cache metrics
	if e.cache != nil {
		stats := e.cache.GetStats()
		sb.WriteString("\n# HELP promptledger_cache_hits_total Total semantic cache hits\n")
		sb.WriteString("# TYPE promptledger_cache_hits_total counter\n")
		sb.WriteString(fmt.Sprintf("promptledger_cache_hits_total %d\n", stats.Hits))

		sb.WriteString("# HELP promptledger_cache_misses_total Total semantic cache misses\n")
		sb.WriteString("# TYPE promptledger_cache_misses_total counter\n")
		sb.WriteString(fmt.Sprintf("promptledger_cache_misses_total %d\n", stats.Misses))

		sb.WriteString("# HELP promptledger_cache_hit_ratio Semantic cache hit ratio\n")
		sb.WriteString("# TYPE promptledger_cache_hit_ratio gauge\n")
		sb.WriteString(fmt.Sprintf("promptledger_cache_hit_ratio %.4f\n", stats.HitRatio))

		sb.WriteString("# HELP promptledger_cache_dollars_saved_usd Cumulative dollars saved via cache\n")
		sb.WriteString("# TYPE promptledger_cache_dollars_saved_usd counter\n")
		sb.WriteString(fmt.Sprintf("promptledger_cache_dollars_saved_usd %.6f\n", stats.TotalDollarsSaved))
	}

	_, _ = io.WriteString(w, sb.String())
}
