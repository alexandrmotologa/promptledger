export interface VariantConfig {
  version: string;
  weight: number;
}

export interface ShadowConfig {
  enabled: boolean;
  version: string;
  sample_rate: number;
}

export interface BanditConfig {
  enabled: boolean;
  epsilon: number;
  quality_weight: number;
  latency_weight: number;
  cost_weight: number;
  min_runs_to_exploit: number;
}

export interface GateConfig {
  min_quality_score: number;
  max_p95_latency_ms: number;
  max_error_rate: number;
  min_sample_size: number;
  auto_promote: boolean;
  auto_rollback: boolean;
}

export interface AssertionSpec {
  type: string;
  pattern?: string;
  banned_words?: string[];
  rubric?: string;
  threshold?: number;
}

export interface BudgetConfig {
  max_daily_cost_usd: number;
  fallback_model: string;
}

export interface CacheConfig {
  enabled: boolean;
  ttl_seconds: number;
}

export interface VersionDetail {
  model: string;
  template: string;
  author: string;
  commit: string;
  description?: string;
  created_at: string;
}

export interface PromptSpec {
  version: string;
  prompt_id: string;
  description: string;
  routing: {
    strategy: string;
    active_version: string;
    variants: VariantConfig[];
    shadow: ShadowConfig;
    bandit?: BanditConfig;
    fallback_version?: string;
  };
  gates: GateConfig;
  evaluations?: AssertionSpec[];
  budget?: BudgetConfig;
  cache?: CacheConfig;
  versions: Record<string, VersionDetail>;
  file_path?: string;
  updated_at?: string;
}

export interface VersionMetric {
  version: string;
  variant_type: string;
  total_runs: number;
  error_count: number;
  error_rate: number;
  avg_latency_ms: number;
  p95_latency_ms: number;
  avg_cost_usd: number;
  total_cost_usd: number;
  avg_quality_score: number;
  semantic_match_pct: number;
}

export interface PromptMetrics {
  prompt_id: string;
  active_version: string;
  total_runs: number;
  versions: Record<string, VersionMetric>;
  shadow_comparisons: number;
  avg_shadow_agreement: number;
  avg_latency_delta_ms: number;
  est_monthly_cost_delta: number;
  auto_promote_candidate: boolean;
  rollback_warning: boolean;
}

export interface SystemSummary {
  total_prompts: number;
  total_executions: number;
  total_shadow_runs: number;
  avg_latency_ms: number;
  total_cost_usd: number;
  avg_shadow_agreement: number;
  active_canaries: number;
}

export interface CacheStats {
  total_lookups: number;
  hits: number;
  misses: number;
  hit_ratio: number;
  total_latency_saved_ms: number;
  total_dollars_saved: number;
  cached_items_count: number;
}

export interface ArmStats {
  trials: number;
  avg_quality: number;
  avg_latency_ms: number;
  avg_cost_usd: number;
  average_reward: number;
}

export interface WaterfallTiming {
  routing_overhead_us: number;
  provider_latency_ms: number;
  total_latency_ms: number;
}

export interface ExecutionRecord {
  id: string;
  prompt_id: string;
  version: string;
  variant_type: 'active' | 'canary' | 'shadow' | 'bandit';
  user_id: string;
  tenant_id: string;
  input_variables: Record<string, any>;
  rendered_prompt: string;
  output_text: string;
  model_used: string;
  latency_ms: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_usd: number;
  is_success: boolean;
  error_message?: string;
  created_at: string;
  from_cache?: boolean;
  budget_downscaled?: boolean;
  timing?: WaterfallTiming;
}

export interface ShadowComparisonRecord {
  id: number;
  prompt_id: string;
  active_execution_id: string;
  shadow_execution_id: string;
  active_version: string;
  shadow_version: string;
  quality_score: number;
  latency_delta_ms: number;
  cost_delta_usd: number;
  semantic_similarity: number;
  verdict: 'better' | 'equivalent' | 'regressed';
  explanation: string;
  created_at: string;
  active_output?: string;
  shadow_output?: string;
  input_json?: string;
}

export interface AuditEventRecord {
  id: number;
  prompt_id: string;
  action: string;
  old_version: string;
  new_version: string;
  reason: string;
  triggered_by: string;
  created_at: string;
}

export interface CaseResult {
  index: number;
  passed: boolean;
  latency_ms: number;
  cost_usd: number;
  output_text: string;
  failure_notes?: string;
}

export interface EvaluationReport {
  prompt_id: string;
  version: string;
  model: string;
  dataset_path: string;
  total_cases: number;
  passed_cases: number;
  failed_cases: number;
  accuracy_rate: number;
  avg_latency_ms: number;
  p95_latency_ms: number;
  total_cost_usd: number;
  execution_time_ms: number;
  results: CaseResult[];
  markdown_summary: string;
}
