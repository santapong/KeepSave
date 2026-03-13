package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	version     = "1.0.0"
	envAPIURL   = "KEEPSAVE_API_URL"
	envAPIKey   = "KEEPSAVE_API_KEY"
	envToken    = "KEEPSAVE_TOKEN"
	envProject  = "KEEPSAVE_PROJECT_ID"
)

type CLI struct {
	apiURL    string
	apiKey    string
	token     string
	projectID string
	client    *http.Client
}

func main() {
	cli := &CLI{
		apiURL:    getEnvOrDefault(envAPIURL, "http://localhost:8080"),
		apiKey:    os.Getenv(envAPIKey),
		token:     os.Getenv(envToken),
		projectID: os.Getenv(envProject),
		client:    &http.Client{},
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "pull":
		err = cli.cmdPull(os.Args[2:])
	case "push":
		err = cli.cmdPush(os.Args[2:])
	case "promote":
		err = cli.cmdPromote(os.Args[2:])
	case "login":
		err = cli.cmdLogin(os.Args[2:])
	case "projects":
		err = cli.cmdProjects(os.Args[2:])
	case "export":
		err = cli.cmdExport(os.Args[2:])
	case "import":
		err = cli.cmdImport(os.Args[2:])
	case "version":
		fmt.Printf("keepsave version %s\n", version)
		return
	case "help", "--help", "-h":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`KeepSave CLI - Secure environment variable management

Usage:
  keepsave <command> [options]

Commands:
  login                     Authenticate with the API server
  pull                      Pull secrets from a project environment
  push                      Push secrets from a .env file to a project environment
  promote                   Promote secrets between environments
  projects                  List projects
  export                    Export secrets as .env file
  import                    Import secrets from .env file
  version                   Print version information
  help                      Show this help message

Environment Variables:
  KEEPSAVE_API_URL          API server URL (default: http://localhost:8080)
  KEEPSAVE_API_KEY          API key for authentication
  KEEPSAVE_TOKEN            JWT token for authentication
  KEEPSAVE_PROJECT_ID       Default project ID

Examples:
  keepsave login
  keepsave pull --project <id> --env alpha
  keepsave push --project <id> --env alpha --file .env
  keepsave promote --project <id> --from alpha --to uat
  keepsave export --project <id> --env prod > .env.prod
  keepsave import --project <id> --env alpha --file .env`)
}

// cmdLogin authenticates with the API and prints the token
func (c *CLI) cmdLogin(args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	body := map[string]string{"email": email, "password": password}
	resp, err := c.doRequest("POST", "/api/v1/auth/login", body)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	token, ok := resp["token"].(string)
	if !ok {
		return fmt.Errorf("no token in response")
	}

	fmt.Println("\nLogin successful!")
	fmt.Printf("Token: %s\n", token)
	fmt.Printf("\nExport it:\n  export %s=%s\n", envToken, token)
	return nil
}

// cmdPull retrieves secrets from a project environment
func (c *CLI) cmdPull(args []string) error {
	projectID, env := c.projectID, "alpha"
	format := "env"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			i++
			if i < len(args) {
				projectID = args[i]
			}
		case "--env", "-e":
			i++
			if i < len(args) {
				env = args[i]
			}
		case "--format", "-f":
			i++
			if i < len(args) {
				format = args[i]
			}
		}
	}

	if projectID == "" {
		return fmt.Errorf("project ID required (--project or KEEPSAVE_PROJECT_ID)")
	}

	resp, err := c.doRequest("GET", fmt.Sprintf("/api/v1/projects/%s/secrets?environment=%s", projectID, env), nil)
	if err != nil {
		return fmt.Errorf("pulling secrets: %w", err)
	}

	secrets, ok := resp["secrets"].([]interface{})
	if !ok {
		fmt.Println("No secrets found.")
		return nil
	}

	switch format {
	case "json":
		out, _ := json.MarshalIndent(secrets, "", "  ")
		fmt.Println(string(out))
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for _, s := range secrets {
			m := s.(map[string]interface{})
			fmt.Fprintf(w, "%s\t%s\n", m["key"], m["value"])
		}
		w.Flush()
	default: // env format
		for _, s := range secrets {
			m := s.(map[string]interface{})
			key, _ := m["key"].(string)
			value, _ := m["value"].(string)
			if strings.ContainsAny(value, " #\n\r\t\"'") {
				value = "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
			}
			fmt.Printf("%s=%s\n", key, value)
		}
	}

	return nil
}

// cmdPush reads a .env file and pushes secrets to a project environment
func (c *CLI) cmdPush(args []string) error {
	projectID, env, file := c.projectID, "alpha", ".env"
	overwrite := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			i++
			if i < len(args) {
				projectID = args[i]
			}
		case "--env", "-e":
			i++
			if i < len(args) {
				env = args[i]
			}
		case "--file", "-f":
			i++
			if i < len(args) {
				file = args[i]
			}
		case "--overwrite":
			overwrite = true
		}
	}

	if projectID == "" {
		return fmt.Errorf("project ID required (--project or KEEPSAVE_PROJECT_ID)")
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", file, err)
	}

	body := map[string]interface{}{
		"environment": env,
		"content":     string(content),
		"overwrite":   overwrite,
	}

	resp, err := c.doRequest("POST", fmt.Sprintf("/api/v1/projects/%s/env-import", projectID), body)
	if err != nil {
		return fmt.Errorf("pushing secrets: %w", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if ok {
		if created, ok := result["created"].([]interface{}); ok && len(created) > 0 {
			fmt.Printf("Created: %d secrets\n", len(created))
		}
		if updated, ok := result["updated"].([]interface{}); ok && len(updated) > 0 {
			fmt.Printf("Updated: %d secrets\n", len(updated))
		}
		if skipped, ok := result["skipped"].([]interface{}); ok && len(skipped) > 0 {
			fmt.Printf("Skipped: %d secrets (already exist, use --overwrite)\n", len(skipped))
		}
	}

	return nil
}

// cmdPromote triggers a promotion between environments
func (c *CLI) cmdPromote(args []string) error {
	projectID := c.projectID
	from, to := "alpha", "uat"
	policy := "overwrite"
	notes := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			i++
			if i < len(args) {
				projectID = args[i]
			}
		case "--from":
			i++
			if i < len(args) {
				from = args[i]
			}
		case "--to":
			i++
			if i < len(args) {
				to = args[i]
			}
		case "--policy":
			i++
			if i < len(args) {
				policy = args[i]
			}
		case "--notes":
			i++
			if i < len(args) {
				notes = args[i]
			}
		}
	}

	if projectID == "" {
		return fmt.Errorf("project ID required (--project or KEEPSAVE_PROJECT_ID)")
	}

	body := map[string]interface{}{
		"source_environment": from,
		"target_environment": to,
		"override_policy":    policy,
		"notes":              notes,
	}

	resp, err := c.doRequest("POST", fmt.Sprintf("/api/v1/projects/%s/promote", projectID), body)
	if err != nil {
		return fmt.Errorf("promoting secrets: %w", err)
	}

	if promo, ok := resp["promotion"].(map[string]interface{}); ok {
		status, _ := promo["status"].(string)
		fmt.Printf("Promotion %s -> %s: %s\n", from, to, status)
		if status == "pending" {
			fmt.Println("Note: PROD promotions require approval.")
		}
	}

	return nil
}

// cmdProjects lists all projects
func (c *CLI) cmdProjects(args []string) error {
	resp, err := c.doRequest("GET", "/api/v1/projects", nil)
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	projects, ok := resp["projects"].([]interface{})
	if !ok || len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")
	fmt.Fprintln(w, "--\t----\t-----------")
	for _, p := range projects {
		m := p.(map[string]interface{})
		fmt.Fprintf(w, "%s\t%s\t%s\n", m["id"], m["name"], m["description"])
	}
	w.Flush()

	return nil
}

// cmdExport exports secrets as .env content
func (c *CLI) cmdExport(args []string) error {
	projectID, env := c.projectID, "alpha"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			i++
			if i < len(args) {
				projectID = args[i]
			}
		case "--env", "-e":
			i++
			if i < len(args) {
				env = args[i]
			}
		}
	}

	if projectID == "" {
		return fmt.Errorf("project ID required (--project or KEEPSAVE_PROJECT_ID)")
	}

	resp, err := c.doRequest("GET", fmt.Sprintf("/api/v1/projects/%s/env-export?environment=%s", projectID, env), nil)
	if err != nil {
		return fmt.Errorf("exporting secrets: %w", err)
	}

	if content, ok := resp["content"].(string); ok {
		fmt.Print(content)
	}

	return nil
}

// cmdImport imports secrets from a .env file
func (c *CLI) cmdImport(args []string) error {
	return c.cmdPush(args)
}

func (c *CLI) doRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.apiURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	if resp.StatusCode == 204 || len(respBody) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
