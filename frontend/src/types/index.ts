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
