package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// characterServiceClient makes authenticated HTTP calls to character-service.
type characterServiceClient struct {
	baseURL    string
	jwtSecret  string
	httpClient *http.Client
}

func newCharacterServiceClient(baseURL, jwtSecret string) *characterServiceClient {
	return &characterServiceClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		jwtSecret:  jwtSecret,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// buildServiceJWT creates a minimal HMAC-SHA256 JWT for service-to-service calls,
// following the same convention used in project-service/episode_service.go.
func buildServiceJWT(secret string, projectID int64) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := map[string]interface{}{
		"user_id":    1,
		"project_id": projectID,
		"role":       "service",
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

// listCharacterNames returns the set of existing character names for a project.
func (c *characterServiceClient) listCharacterNames(ctx context.Context, projectID int64) (map[string]bool, error) {
	token, err := buildServiceJWT(c.jwtSecret, projectID)
	if err != nil {
		return nil, fmt.Errorf("build service jwt: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/characters?project_id=%d", c.baseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list characters status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse list response: %w", err)
	}

	names := make(map[string]bool, len(result.Data))
	for _, ch := range result.Data {
		names[ch.Name] = true
	}
	return names, nil
}

// createCharacter creates a single character in character-service.
// Returns nil on success or 409 (already exists).
func (c *characterServiceClient) createCharacter(ctx context.Context, projectID int64, info CharacterExtractInfo) error {
	token, err := buildServiceJWT(c.jwtSecret, projectID)
	if err != nil {
		return fmt.Errorf("build service jwt: %w", err)
	}

	payload := map[string]interface{}{
		"project_id":      projectID,
		"name":            info.Name,
		"role_desc":       info.RoleDesc,
		"appearance_desc": info.AppearanceDesc,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal character: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/characters", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create character request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create character: %w", err)
	}
	defer resp.Body.Close()

	// 201 Created or 200 OK → success; 409 Conflict → already exists, skip.
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusConflict {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create character status %d: %s", resp.StatusCode, respBody)
}
