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
