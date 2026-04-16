// Phase 15: AI Intelligence types

export interface AIProviderStatus {
  provider: string;
  available: boolean;
  model: string;
  error?: string;
}

export interface DriftCheck {
  id: string;
  project_id: string;
  source_env: string;
  target_env: string;
  status: string;
  total_keys: number;
  drifted_keys: number;
  missing_in_source: number;
  missing_in_target: number;
  drift_entries: DriftDetailEntry[] | Record<string, unknown>;
  remediation: string;
  created_at: string;
  completed_at?: string;
}

export interface DriftDetailEntry {
  key: string;
  drift_type: 'value_mismatch' | 'missing_source' | 'missing_target';
  source_exists: boolean;
  target_exists: boolean;
  values_differ: boolean;
  recommendation: string;
}

export interface Anomaly {
  id: string;
  project_id?: string;
  api_key_id?: string;
  anomaly_type: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  description: string;
  details: Record<string, unknown>;
  status: 'open' | 'acknowledged' | 'resolved' | 'false_positive';
  detected_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
}

export interface UsageTrend {
  period: string;
  date: string;
  access_count: number;
  unique_users: number;
  unique_keys: number;
  growth_percent: number;
}

export interface UsageForecast {
  date: string;
  predicted_count: number;
  lower_bound: number;
  upper_bound: number;
  confidence: number;
}

export interface SecretRecommendation {
  id: string;
  project_id: string;
  recomm_type: string;
  severity: 'info' | 'warning' | 'critical';
  title: string;
  description: string;
  affected_keys: string[];
  suggested_action: string;
  auto_fixable: boolean;
  status: 'pending' | 'applied' | 'dismissed';
  created_at: string;
}

export interface NLPQueryResult {
  query: string;
  intent: string;
  matched_secrets: NLPSecretMatch[];
  explanation: string;
  suggestions: string[];
  provider: string;
  model: string;
}

export interface NLPSecretMatch {
  project_id: string;
  project_name: string;
  environment: string;
  key: string;
  score: number;
  reason: string;
}
