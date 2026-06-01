package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

type VolcAssetClientConfig struct {
	Enabled         bool
	AccessKey       string
	SecretKey       string
	Region          string
	Service         string
	Host            string
	Version         string
	ProjectName     string
	GroupType       string
	GroupNamePrefix string
	PollInterval    time.Duration
	PollTimeout     time.Duration
	HTTPClient      *http.Client
}

type VolcAssetClient struct {
	accessKey       string
	secretKey       string
	region          string
	service         string
	host            string
	version         string
	projectName     string
	groupType       string
	groupNamePrefix string
	pollInterval    time.Duration
	pollTimeout     time.Duration
	httpClient      *http.Client
	log             *zap.Logger
}

type VolcAssetResult struct {
	ID          string `json:"Id"`
	AssetType   string `json:"AssetType"`
	GroupID     string `json:"GroupId"`
	Status      string `json:"Status"`
	ProjectName string `json:"ProjectName"`
	Name        string `json:"Name"`
	URL         string `json:"URL"`
	CreateTime  string `json:"CreateTime"`
	UpdateTime  string `json:"UpdateTime"`
}

type volcAssetEnvelope struct {
	ResponseMetadata struct {
		RequestID string `json:"RequestId"`
		Action    string `json:"Action"`
		Version   string `json:"Version"`
		Service   string `json:"Service"`
		Region    string `json:"Region"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"ResponseMetadata"`
	Result *VolcAssetResult `json:"Result"`
}

func NewVolcAssetClient(cfg VolcAssetClientConfig, log *zap.Logger) *VolcAssetClient {
	if !cfg.Enabled {
		return nil
	}
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if accessKey == "" || secretKey == "" {
		return nil
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "cn-beijing"
	}
	serviceName := strings.TrimSpace(cfg.Service)
	if serviceName == "" {
		serviceName = "ark"
	}
	host := normalizeVolcAssetHost(cfg.Host)
	if host == "" {
		host = "ark.cn-beijing.volcengineapi.com"
	}
	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = "2024-01-01"
	}
	projectName := strings.TrimSpace(cfg.ProjectName)
	if projectName == "" {
		projectName = "default"
	}
	groupType := strings.TrimSpace(cfg.GroupType)
	if groupType == "" {
		groupType = "AIGC"
	}
	groupNamePrefix := strings.TrimSpace(cfg.GroupNamePrefix)
	if groupNamePrefix == "" {
		groupNamePrefix = "autovideo-character"
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	pollTimeout := cfg.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = 90 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &VolcAssetClient{
		accessKey:       accessKey,
		secretKey:       secretKey,
		region:          region,
		service:         serviceName,
		host:            host,
		version:         version,
		projectName:     projectName,
		groupType:       groupType,
		groupNamePrefix: groupNamePrefix,
		pollInterval:    pollInterval,
		pollTimeout:     pollTimeout,
		httpClient:      httpClient,
		log:             log,
	}
}

func normalizeVolcAssetHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	return strings.TrimRight(host, "/")
}

func (c *VolcAssetClient) Enabled() bool {
	return c != nil && c.accessKey != "" && c.secretKey != ""
}

func (c *VolcAssetClient) ProjectName() string {
	if c == nil {
		return ""
	}
	return c.projectName
}

func (c *VolcAssetClient) GroupNamePrefix() string {
	if c == nil {
		return ""
	}
	return c.groupNamePrefix
}

func (c *VolcAssetClient) CreateAssetGroup(ctx context.Context, name, description string) (*VolcAssetResult, error) {
	body := map[string]interface{}{
		"Name":        strings.TrimSpace(name),
		"Description": strings.TrimSpace(description),
		"ProjectName": c.projectName,
	}
	if groupType := strings.TrimSpace(c.groupType); groupType != "" {
		body["GroupType"] = groupType
	}
	return c.call(ctx, "CreateAssetGroup", body)
}

func (c *VolcAssetClient) CreateAsset(ctx context.Context, groupID, sourceURL, assetType string) (*VolcAssetResult, error) {
	body := map[string]interface{}{
		"GroupId":     strings.TrimSpace(groupID),
		"URL":         strings.TrimSpace(sourceURL),
		"AssetType":   firstNonEmptyString(strings.TrimSpace(assetType), "Image"),
		"ProjectName": c.projectName,
	}
	return c.call(ctx, "CreateAsset", body)
}

func (c *VolcAssetClient) GetAsset(ctx context.Context, assetID string) (*VolcAssetResult, error) {
	body := map[string]interface{}{
		"Id":          strings.TrimSpace(assetID),
		"ProjectName": c.projectName,
	}
	return c.call(ctx, "GetAsset", body)
}

func (c *VolcAssetClient) WaitForAssetActive(ctx context.Context, assetID string) (*VolcAssetResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("volc asset client not configured")
	}
	if c.pollTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.pollTimeout)
			defer cancel()
		}
	}
	for {
		result, err := c.GetAsset(ctx, assetID)
		if err != nil {
			return nil, err
		}
		status := strings.TrimSpace(strings.ToLower(result.Status))
		switch status {
		case "active":
			return result, nil
		case "failed":
			return nil, fmt.Errorf("provider asset processing failed")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.pollInterval):
		}
	}
}

func (c *VolcAssetClient) call(ctx context.Context, action string, body interface{}) (*VolcAssetResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("volc asset client not configured")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", action, err)
	}
	requestURL := c.signedURL(action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", action, err)
	}
	contentHash := sha256Hex(payload)
	xDate, shortDate := volcDates(time.Now().UTC())
	credentialScope := fmt.Sprintf("%s/%s/%s/request", shortDate, c.region, c.service)
	signedHeaders := "content-type;host;x-content-sha256;x-date"
	req.Header.Set("Host", c.host)
	req.Header.Set("X-Date", xDate)
	req.Header.Set("X-Content-Sha256", contentHash)
	req.Header.Set("Content-Type", "application/json")
	canonicalRequest := buildVolcCanonicalRequest(http.MethodPost, "/", action, c.version, c.host, xDate, contentHash, signedHeaders)
	stringToSign := buildVolcStringToSign(xDate, credentialScope, sha256Hex([]byte(canonicalRequest)))
	signingKey := volcSigningKey(c.secretKey, shortDate, c.region, c.service)
	signature := hmacSHA256Hex(signingKey, stringToSign)
	req.Header.Set("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", c.accessKey, credentialScope, signedHeaders, signature))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", action, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var envelope volcAssetEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", action, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s failed (%d): %s", action, resp.StatusCode, compactVolcMessage(envelope, respBody))
	}
	if envelope.ResponseMetadata.Error != nil {
		return nil, fmt.Errorf("%s failed: %s", action, firstNonEmptyString(envelope.ResponseMetadata.Error.Message, envelope.ResponseMetadata.Error.Code))
	}
	if envelope.Result == nil || strings.TrimSpace(envelope.Result.ID) == "" {
		return nil, fmt.Errorf("%s returned empty result", action)
	}
	return envelope.Result, nil
}

func compactVolcMessage(envelope volcAssetEnvelope, raw []byte) string {
	if envelope.ResponseMetadata.Error != nil {
		return firstNonEmptyString(envelope.ResponseMetadata.Error.Message, envelope.ResponseMetadata.Error.Code)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "empty response"
	}
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func (c *VolcAssetClient) signedURL(action string) string {
	query := url.Values{}
	query.Set("Action", action)
	query.Set("Version", c.version)
	return fmt.Sprintf("https://%s/?%s", c.host, buildVolcQuery(query))
}

func buildVolcCanonicalRequest(method, path, action, version, host, xDate, contentHash, signedHeaders string) string {
	query := url.Values{}
	query.Set("Action", action)
	query.Set("Version", version)
	canonicalQuery := buildVolcQuery(query)
	return strings.Join([]string{
		method,
		path,
		canonicalQuery,
		"content-type:application/json",
		"host:" + host,
		"x-content-sha256:" + contentHash,
		"x-date:" + xDate,
		"",
		signedHeaders,
		contentHash,
	}, "\n")
}

func buildVolcStringToSign(xDate, credentialScope, canonicalRequestHash string) string {
	return strings.Join([]string{
		"HMAC-SHA256",
		xDate,
		credentialScope,
		canonicalRequestHash,
	}, "\n")
}

func buildVolcQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, volcQueryEscape(key)+"="+volcQueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func volcQueryEscape(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch)
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		default:
			b.WriteString(fmt.Sprintf("%%%02X", ch))
		}
	}
	return b.String()
}

func volcDates(now time.Time) (string, string) {
	xDate := now.UTC().Format("20060102T150405Z")
	return xDate, xDate[:8]
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

func volcSigningKey(secretKey, shortDate, region, service string) []byte {
	kDate := hmacSHA256([]byte(secretKey), shortDate)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "request")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
