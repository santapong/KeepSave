package api

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Description string `json:"description"`
}

type UpdateProjectRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Description string `json:"description"`
}

type CreateSecretRequest struct {
	Key         string `json:"key" binding:"required,min=1,max=255"`
	Value       string `json:"value" binding:"required"`
	Environment string `json:"environment" binding:"required,oneof=alpha uat prod"`
}

type UpdateSecretRequest struct {
	Value string `json:"value" binding:"required"`
}

type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	ProjectID   string   `json:"project_id" binding:"required,uuid"`
	Scopes      []string `json:"scopes" binding:"omitempty,dive,oneof=read write delete promote"`
	Environment *string  `json:"environment" binding:"omitempty,oneof=alpha uat prod"`
}

type PromoteRequest struct {
	SourceEnvironment string   `json:"source_environment" binding:"required,oneof=alpha uat"`
	TargetEnvironment string   `json:"target_environment" binding:"required,oneof=uat prod"`
	Keys              []string `json:"keys"`
	OverridePolicy    string   `json:"override_policy" binding:"omitempty,oneof=skip overwrite"`
	Notes             string   `json:"notes"`
}

type DiffRequest struct {
	SourceEnvironment string   `json:"source_environment" binding:"required,oneof=alpha uat"`
	TargetEnvironment string   `json:"target_environment" binding:"required,oneof=uat prod"`
	Keys              []string `json:"keys"`
}

// Phase 6 request types

type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type UpdateOrganizationRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
	Role   string `json:"role" binding:"required,oneof=viewer editor admin promoter"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=viewer editor admin promoter"`
}

type AssignProjectRequest struct {
	ProjectID string `json:"project_id" binding:"required,uuid"`
}

type CreateTemplateRequest struct {
	Name           string                 `json:"name" binding:"required,min=1,max=255"`
	Description    string                 `json:"description"`
	Stack          string                 `json:"stack" binding:"required"`
	Keys           map[string]interface{} `json:"keys" binding:"required"`
	OrganizationID string                 `json:"organization_id"`
	IsGlobal       bool                   `json:"is_global"`
}

type UpdateTemplateRequest struct {
	Name        string                 `json:"name" binding:"required,min=1,max=255"`
	Description string                 `json:"description"`
	Stack       string                 `json:"stack" binding:"required"`
	Keys        map[string]interface{} `json:"keys" binding:"required"`
}

type ApplyTemplateRequest struct {
	ProjectID   string `json:"project_id" binding:"required,uuid"`
	Environment string `json:"environment" binding:"required,oneof=alpha uat prod"`
}

type ImportEnvRequest struct {
	Environment string `json:"environment" binding:"required,oneof=alpha uat prod"`
	Content     string `json:"content" binding:"required"`
	Overwrite   bool   `json:"overwrite"`
}

// Phase 13: OAuth & MCP Hub request types

type RegisterOAuthClientRequest struct {
	Name         string   `json:"name" binding:"required,min=1,max=255"`
	Description  string   `json:"description"`
	RedirectURIs []string `json:"redirect_uris" binding:"required"`
	Scopes       []string `json:"scopes"`
	GrantTypes   []string `json:"grant_types"`
	LogoURL      string   `json:"logo_url"`
	HomepageURL  string   `json:"homepage_url"`
	IsPublic     bool     `json:"is_public"`
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type RegisterMCPServerRequest struct {
	Name         string                 `json:"name" binding:"required,min=1,max=255"`
	Description  string                 `json:"description"`
	GitHubURL    string                 `json:"github_url" binding:"required"`
	GitHubBranch string                 `json:"github_branch"`
	EntryCommand string                 `json:"entry_command"`
	Transport    string                 `json:"transport"`
	IconURL      string                 `json:"icon_url"`
	Version      string                 `json:"version"`
	EnvMappings  map[string]interface{} `json:"env_mappings"`
	IsPublic     bool                   `json:"is_public"`
}

type UpdateMCPServerRequest struct {
	Name         string                 `json:"name" binding:"required,min=1,max=255"`
	Description  string                 `json:"description"`
	GitHubBranch string                 `json:"github_branch"`
	EntryCommand string                 `json:"entry_command"`
	Transport    string                 `json:"transport"`
	IconURL      string                 `json:"icon_url"`
	Version      string                 `json:"version"`
	EnvMappings  map[string]interface{} `json:"env_mappings"`
	IsPublic     bool                   `json:"is_public"`
}

type InstallMCPServerRequest struct {
	MCPServerID string                 `json:"mcp_server_id" binding:"required,uuid"`
	ProjectID   string                 `json:"project_id"`
	Config      map[string]interface{} `json:"config"`
}

type UpdateInstallationRequest struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}
