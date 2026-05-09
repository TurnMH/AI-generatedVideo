package generators

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
	"strconv"
	"strings"
	"time"
)

// TencentVCLMGenerator wraps the Tencent Cloud VCLM image-to-video API.
// Uses TC3-HMAC-SHA256 signing (same credentials as Tencent Cloud VOD).
//
// Submit:  POST https://vclm.tencentcloudapi.com/  X-TC-Action: SubmitImageToVideoGeneralJob
// Query:   POST https://vclm.tencentcloudapi.com/  X-TC-Action: DescribeImageToVideoJob
//
// Note: The free/basic tier allows only 1 concurrent job. The generator uses a
// package-level semaphore to serialise submissions across all clips/tasks.
type TencentVCLMGenerator struct {
	SecretID  string // Tencent Cloud SecretId (AKIDxxx)
	SecretKey string // Tencent Cloud SecretKey
	Region    string // e.g. "ap-guangzhou"
	client    *http.Client
}

// vclmSubmitSem serialises VCLM submissions: free tier allows 1 concurrent job.
var vclmSubmitSem = make(chan struct{}, 1)

const (
	vclmHost    = "vclm.tencentcloudapi.com"
	vclmService = "vclm"
	vclmVersion = "2024-05-23"
)

// NewTencentVCLMGenerator creates a Tencent VCLM generator.
func NewTencentVCLMGenerator(secretID, secretKey, region string) *TencentVCLMGenerator {
	if region == "" {
		region = "ap-guangzhou"
	}
	return &TencentVCLMGenerator{
		SecretID:  secretID,
		SecretKey: secretKey,
		Region:    region,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *TencentVCLMGenerator) Name() string             { return "tencent-vclm" }
func (g *TencentVCLMGenerator) IsAvailable(_ context.Context) bool {
	return g.SecretID != "" && g.SecretKey != ""
}
func (g *TencentVCLMGenerator) SupportsNativeAudio() bool { return false }
func (g *TencentVCLMGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "resolution", Label: "分辨率", Default: "720p",
			Values: []ParamValue{
				{Value: "480p", Label: "480p"},
				{Value: "720p", Label: "720p"},
				{Value: "1080p", Label: "1080p"},
			},
		},
	}
}

// ── API request/response types ─────────────────────────────────────────────────

type vclmImageRef struct {
	Url string `json:"Url"`
}

type vclmSubmitReq struct {
	Image      vclmImageRef `json:"Image"`
	Prompt     string       `json:"Prompt,omitempty"`
	Resolution string       `json:"Resolution,omitempty"` // "480p" / "720p" / "1080p"
}

type vclmSubmitResp struct {
	Response struct {
		JobId     string `json:"JobId"`
		RequestId string `json:"RequestId"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"Response"`
}

type vclmQueryReq struct {
	JobId string `json:"JobId"`
}

type vclmQueryResp struct {
	Response struct {
		Status         string `json:"Status"` // PROCESSING / DONE / FAILED
		ResultVideoUrl string `json:"ResultVideoUrl"`
		ErrorCode      string `json:"ErrorCode"`
		ErrorMessage   string `json:"ErrorMessage"`
		RequestId      string `json:"RequestId"`
		Error          *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"Response"`
}

// ── Generate ──────────────────────────────────────────────────────────────────

func (g *TencentVCLMGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	resolution := firstNonEmpty(req.Resolution, "720p")

	// Acquire the per-process concurrency slot for the ENTIRE generate cycle
	// (submit + poll). Tencent free tier allows only 1 concurrent job globally,
	// so we must wait for the previous job to finish before submitting the next.
	select {
	case vclmSubmitSem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-vclmSubmitSem }()

	jobID, err := g.submitJob(ctx, req.SourceImageURL, req.Prompt, resolution)
	if err != nil {
		return nil, fmt.Errorf("vclm submit: %w", err)
	}

	clip, err := g.pollJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("vclm poll %s: %w", jobID, err)
	}
	return clip, nil
}

func (g *TencentVCLMGenerator) submitJob(ctx context.Context, imageURL, prompt, resolution string) (string, error) {
	// Tencent VCLM limits Prompt to 200 characters.
	if len([]rune(prompt)) > 200 {
		runes := []rune(prompt)
		prompt = string(runes[:200])
	}
	body := vclmSubmitReq{
		Image:      vclmImageRef{Url: imageURL},
		Prompt:     prompt,
		Resolution: resolution,
	}
	b, _ := json.Marshal(body)

	resp, err := g.doRequest(ctx, "SubmitImageToVideoGeneralJob", b)
	if err != nil {
		return "", err
	}

	var result vclmSubmitResp
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(resp))
	}
	if result.Response.Error != nil {
		return "", fmt.Errorf("vclm error %s: %s", result.Response.Error.Code, result.Response.Error.Message)
	}
	if result.Response.JobId == "" {
		return "", fmt.Errorf("vclm: no JobId in response: %s", string(resp))
	}
	return result.Response.JobId, nil
}

func (g *TencentVCLMGenerator) pollJob(ctx context.Context, jobID string) (*VideoClip, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	timeout := time.After(20 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("vclm: job %s timed out after 20min", jobID)
		case <-ticker.C:
			clip, done, err := g.queryJob(ctx, jobID)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

func (g *TencentVCLMGenerator) queryJob(ctx context.Context, jobID string) (*VideoClip, bool, error) {
	body := vclmQueryReq{JobId: jobID}
	b, _ := json.Marshal(body)

	resp, err := g.doRequest(ctx, "DescribeImageToVideoJob", b)
	if err != nil {
		return nil, false, nil // transient, retry
	}

	var result vclmQueryResp
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, false, nil // transient
	}
	if result.Response.Error != nil {
		return nil, false, fmt.Errorf("vclm: %s %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	switch result.Response.Status {
	case "DONE":
		if result.Response.ResultVideoUrl == "" {
			return nil, false, fmt.Errorf("vclm: DONE but no ResultVideoUrl")
		}
		return &VideoClip{
			ClipURL:     result.Response.ResultVideoUrl,
			DurationSec: 5,
			ModelUsed:   g.Name(),
		}, true, nil
	case "FAILED":
		msg := result.Response.ErrorMessage
		if msg == "" {
			msg = result.Response.ErrorCode
		}
		return nil, false, fmt.Errorf("vclm: job %s failed: %s", jobID, msg)
	default: // PROCESSING or empty
		return nil, false, nil
	}
}

// ── TC3-HMAC-SHA256 signing ───────────────────────────────────────────────────

func (g *TencentVCLMGenerator) doRequest(ctx context.Context, action string, payload []byte) ([]byte, error) {
	now := time.Now().UTC()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	date := now.Format("2006-01-02")

	// Headers that must be signed (sorted alphabetically)
	contentType := "application/json; charset=utf-8"
	actionLower := strings.ToLower(action)
	signedHeaders := "content-type;host;x-tc-action"

	// Step 1: canonical request
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := "content-type:" + contentType + "\n" +
		"host:" + vclmHost + "\n" +
		"x-tc-action:" + actionLower + "\n"
	canonicalRequest := strings.Join([]string{
		"POST", "/", "",
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	// Step 2: string to sign
	credentialScope := date + "/" + vclmService + "/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + timestamp + "\n" + credentialScope + "\n" + sha256Hex([]byte(canonicalRequest))

	// Step 3: derive signing key and sign
	secretDate := hmacSHA256([]byte("TC3"+g.SecretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(vclmService))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	// Step 4: authorization header
	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		g.SecretID, credentialScope, signedHeaders, signature,
	)

	endpoint := "https://" + vclmHost + "/"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Host", vclmHost)
	httpReq.Header.Set("X-TC-Action", action)
	httpReq.Header.Set("X-TC-Version", vclmVersion)
	httpReq.Header.Set("X-TC-Timestamp", timestamp)
	httpReq.Header.Set("X-TC-Region", g.Region)
	httpReq.Header.Set("Authorization", authorization)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vclm HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
