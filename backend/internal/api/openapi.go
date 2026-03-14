package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OpenAPIHandler serves the API documentation.
type OpenAPIHandler struct{}

// NewOpenAPIHandler creates a new OpenAPI handler.
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

// Spec serves the OpenAPI 3.0 specification.
func (h *OpenAPIHandler) Spec(c *gin.Context) {
	c.JSON(http.StatusOK, openAPISpec())
}

func openAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "KeepSave API",
			"description": "Secure environment variable storage and promotion system for AI Agents and development teams.",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "KeepSave Team",
			},
		},
		"servers": []map[string]interface{}{
			{"url": "/api/v1", "description": "API v1"},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKeyAuth": map[string]interface{}{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
			},
			"schemas": map[string]interface{}{
				"Project": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":          map[string]string{"type": "string", "format": "uuid"},
						"name":        map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
						"owner_id":    map[string]string{"type": "string", "format": "uuid"},
						"created_at":  map[string]string{"type": "string", "format": "date-time"},
						"updated_at":  map[string]string{"type": "string", "format": "date-time"},
					},
				},
				"Secret": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":             map[string]string{"type": "string", "format": "uuid"},
						"project_id":     map[string]string{"type": "string", "format": "uuid"},
						"environment_id": map[string]string{"type": "string", "format": "uuid"},
						"key":            map[string]string{"type": "string"},
						"value":          map[string]string{"type": "string"},
						"created_at":     map[string]string{"type": "string", "format": "date-time"},
						"updated_at":     map[string]string{"type": "string", "format": "date-time"},
					},
				},
				"Error": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"code":    map[string]string{"type": "integer"},
								"message": map[string]string{"type": "string"},
							},
						},
					},
				},
			},
		},
		"paths": map[string]interface{}{
			"/auth/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Register a new user",
					"tags":    []string{"Authentication"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email", "password"},
									"properties": map[string]interface{}{
										"email":    map[string]string{"type": "string", "format": "email"},
										"password": map[string]string{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]string{"description": "User registered successfully"},
						"400": map[string]string{"description": "Invalid request"},
					},
				},
			},
			"/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Login and get JWT token",
					"tags":    []string{"Authentication"},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "Login successful"},
						"401": map[string]string{"description": "Invalid credentials"},
					},
				},
			},
			"/projects": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List all projects",
					"tags":     []string{"Projects"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "List of projects"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create a new project",
					"tags":     []string{"Projects"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]string{"description": "Project created"},
					},
				},
			},
			"/projects/{id}/secrets": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "List secrets for a project",
					"tags":    []string{"Secrets"},
					"security": []map[string]interface{}{
						{"bearerAuth": []string{}},
						{"apiKeyAuth": []string{}},
					},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]string{"type": "string"}},
						{"name": "environment", "in": "query", "required": true, "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "List of secrets"},
					},
				},
			},
			"/projects/{id}/promote": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Promote secrets between environments",
					"tags":     []string{"Promotions"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "Promotion initiated"},
					},
				},
			},
			"/projects/{id}/rotate-keys": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Rotate project encryption keys",
					"tags":     []string{"Key Rotation"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "Keys rotated"},
					},
				},
			},
			"/organizations": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List organizations",
					"tags":     []string{"Organizations"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "List of organizations"},
					},
				},
			},
			"/templates": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List secret templates",
					"tags":     []string{"Templates"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "List of templates"},
					},
				},
			},
			"/api-keys": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List API keys",
					"tags":     []string{"API Keys"},
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]string{"description": "List of API keys"},
					},
				},
			},
		},
	}
}
