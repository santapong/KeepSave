export interface User {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
}

export interface Environment {
  id: string;
  project_id: string;
  name: string;
  created_at: string;
}

export interface Secret {
  id: string;
  project_id: string;
  environment_id: string;
  key: string;
  value?: string;
  created_at: string;
  updated_at: string;
}

export interface APIKey {
  id: string;
  name: string;
  user_id: string;
  project_id: string;
  scopes: string[];
  environment?: string;
  expires_at?: string;
  created_at: string;
}

export interface AuditEntry {
  id: string;
  user_id?: string;
  project_id?: string;
  action: string;
  environment?: string;
  details: Record<string, unknown>;
  ip_address: string;
  created_at: string;
}

export interface PromotionRequest {
  id: string;
  project_id: string;
  source_environment: string;
  target_environment: string;
  status: 'pending' | 'approved' | 'rejected' | 'completed';
  requested_by: string;
  approved_by?: string;
  keys_filter?: string[];
  override_policy: string;
  notes?: string;
  created_at: string;
  completed_at?: string;
}

export interface DiffEntry {
  key: string;
  action: 'add' | 'update' | 'no_change';
  source_value?: string;
  target_value?: string;
  source_exists: boolean;
  target_exists: boolean;
}

// Phase 6 types

export interface Organization {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
}

export interface OrgMember {
  id: string;
  organization_id: string;
  user_id: string;
  role: 'viewer' | 'editor' | 'admin' | 'promoter';
  created_at: string;
  updated_at: string;
}

export interface SecretTemplate {
  id: string;
  name: string;
  description: string;
  stack: string;
  keys: Record<string, unknown>;
  created_by: string;
  organization_id?: string;
  is_global: boolean;
  created_at: string;
  updated_at: string;
}

export interface SecretDependency {
  id: string;
  project_id: string;
  environment_id: string;
  secret_key: string;
  depends_on_key: string;
  reference_pattern: string;
  created_at: string;
}

export interface DependencyNode {
  key: string;
  depends_on: string[];
  referenced_by: string[];
}

export interface ImportResult {
  created: string[];
  updated: string[];
  skipped: string[];
}

// Phase 14: Application Dashboard types

export interface DashboardApplication {
  id: string;
  name: string;
  url: string;
  description: string;
  icon: string;
  category: string;
  owner_id: string;
  is_favorite: boolean;
  created_at: string;
  updated_at: string;
}

// Phase 14: Dashboard types

export interface SystemHealth {
  status: string;
  version?: string;
  uptime?: string;
  health_url?: string;
  ready_url?: string;
  metrics_url?: string;
  total_projects?: number;
  total_users?: number;
  total_secrets?: number;
  total_api_keys?: number;
  database?: { status: string; connections?: number };
  encryption?: { status: string };
}

export interface TraceSpan {
  trace_id: string;
  span_id: string;
  operation: string;
  status: string;
  duration: string;
  start_time?: string;
  attributes?: Record<string, string>;
}

export interface PlatformEvent {
  id: string;
  event_type: string;
  aggregate_id: string;
  aggregate_type: string;
  payload: Record<string, unknown>;
  published: boolean;
  created_at: string;
}

export interface Plugin {
  id: string;
  name: string;
  plugin_type: string;
  version: string;
  enabled: boolean;
  config: Record<string, unknown>;
  created_at: string;
}

export interface SecurityEvent {
  id: string;
  event_type: string;
  severity: 'info' | 'warning' | 'critical';
  ip_address: string;
  user_id?: string;
  user_agent?: string;
  details: Record<string, unknown>;
  created_at: string;
}

export interface AgentActivity {
  api_key_id: string;
  api_key_name: string;
  project_id: string;
  action: string;
  secret_key?: string;
  environment?: string;
  ip_address: string;
  created_at: string;
}

export interface HeatmapEntry {
  hour: number;
  day: number;
  count: number;
}

export interface AccessPolicy {
  id: string;
  project_id: string;
  policy_type: string;
  config: Record<string, unknown>;
  created_at: string;
}

export interface WebhookDelivery {
  id: string;
  webhook_url: string;
  event_type: string;
  status: 'success' | 'failed' | 'pending';
  status_code?: number;
  response_body?: string;
  attempts: number;
  created_at: string;
}
