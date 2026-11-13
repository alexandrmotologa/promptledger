export interface VariantConfig {
  version: string;
  weight: number;
}

export interface ShadowConfig {
  enabled: boolean;
  version: string;
  sample_rate: number;
}

export interface GateConfig {
  min_quality_score: number;
  max_p95_latency_ms: number;
  max_error_rate: number;
  min_sample_size: number;
  auto_promote: boolean;
  auto_rollback: boolean;
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
  };
  gates: GateConfig;
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

export interface ExecutionRecord {
  id: string;
  prompt_id: string;
  version: string;
  variant_type: 'active' | 'canary' | 'shadow';
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
