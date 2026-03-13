package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	MasterKey   []byte
	JWTSecret   string
	Port        string
	CORSOrigins string
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	masterKeyB64 := os.Getenv("MASTER_KEY")
	if masterKeyB64 == "" {
		return nil, fmt.Errorf("MASTER_KEY is required")
	}
	masterKey, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decoding MASTER_KEY: %w", err)
	}
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("MASTER_KEY must be exactly 32 bytes, got %d", len(masterKey))
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "*"
	}

	return &Config{
		DatabaseURL: databaseURL,
		MasterKey:   masterKey,
		JWTSecret:   jwtSecret,
		Port:        port,
		CORSOrigins: corsOrigins,
	}, nil
}
