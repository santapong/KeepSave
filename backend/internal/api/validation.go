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
	Scopes      []string `json:"scopes"`
	Environment *string  `json:"environment"`
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
