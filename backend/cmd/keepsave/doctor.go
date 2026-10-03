package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// doctor only queries the operator readiness projection. It does not probe
// providers, send mail, decrypt records or change installation state.
func (c *CLI) cmdDoctor(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: keepsave doctor")
	}
	if c.token == "" {
		return fmt.Errorf("operator sign-in token required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.apiURL, "/")+"/api/v1/operator/readiness", nil)
	if err != nil {
		return fmt.Errorf("invalid application URL")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("readiness endpoint unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("readiness denied or unavailable (HTTP %d)", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return fmt.Errorf("invalid readiness response")
	}
	var report any
	if err = json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("invalid readiness response")
	}
	formatted, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(formatted))
	return nil
}
