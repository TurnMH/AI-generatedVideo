package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RunningHubGenerator drives the RunningHub cloud ComfyUI API as a fallback
// when the local ComfyUI instance is unavailable (opt-p3).
//
// RunningHub API overview:
//   POST /task/openapi/create  → { taskId }
//   GET  /task/openapi/status/{taskId} → { taskStatus, outputs: [{fileUrl}] }
//
// Docs: https://www.runninghub.cn/doc/openapi
type RunningHubGenerator struct {
	APIKey     string
	BaseURL    string
	WorkflowID string // RunningHub workflow (app) ID
	NodeID     string // node ID for image input (default "1")
	client     *http.Client
}

// NewRunningHubGenerator creates a RunningHub generator.
func NewRunningHubGenerator(apiKey, baseURL, workflowID, nodeID string) *RunningHubGenerator {
	if baseURL == "" {
		baseURL = "https://www.runninghub.cn"
	}
	if nodeID == "" {
		nodeID = "1"
	}
	return &RunningHubGenerator{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		WorkflowID: workflowID,
		NodeID:     nodeID,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *RunningHubGenerator) Name() string                         { return "runninghub" }
func (g *RunningHubGenerator) SupportsNativeAudio() bool            { return false }

// ParamOptions —— RunningHub 由工作流决定，无固定可配参数
func (g *RunningHubGenerator) ParamOptions() []ModelParamOption { return nil }
func (g *RunningHubGenerator) IsAvailable(ctx context.Context) bool { return g.APIKey != "" && g.WorkflowID != "" }

// ── API types ─────────────────────────────────────────────────────────────────

type rhNodeInput struct {
	NodeID    string `json:"nodeId"`
	FieldName string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

type rhCreateReq struct {
	APIKey     string        `json:"apiKey"`
	WorkflowID string        `json:"workflowId"`
	NodeInfoList []rhNodeInput `json:"nodeInfoList"`
}

type rhCreateResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type rhStatusResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskStatus string `json:"taskStatus"` // RUNNING | SUCCESS | FAILED
		Outputs    []struct {
			FileURL string `json:"fileUrl"`
		} `json:"outputs"`
	} `json:"data"`
}

// ── Generate ──────────────────────────────────────────────────────────────────

func (g *RunningHubGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	taskID, err := g.submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("runninghub submit: %w", err)
	}
	return g.poll(ctx, taskID)
}

func (g *RunningHubGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	body := rhCreateReq{
		APIKey:     g.APIKey,
		WorkflowID: g.WorkflowID,
		NodeInfoList: []rhNodeInput{
			{NodeID: g.NodeID, FieldName: "image", FieldValue: req.SourceImageURL},
			{NodeID: g.NodeID, FieldName: "prompt", FieldValue: req.Prompt},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/task/openapi/create", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result rhCreateResp
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}
	if result.Code != 0 || result.Data.TaskID == "" {
		return "", fmt.Errorf("runninghub create failed: %s (code %d)", result.Msg, result.Code)
	}
	return result.Data.TaskID, nil
}

func (g *RunningHubGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	deadline := time.Now().Add(15 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("runninghub: task %s timed out after 15 minutes", taskID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(8 * time.Second):
		}

		status, err := g.fetchStatus(ctx, taskID)
		if err != nil {
			continue // transient — retry
		}
		switch status.Data.TaskStatus {
		case "SUCCESS":
			if len(status.Data.Outputs) == 0 {
				return nil, fmt.Errorf("runninghub: task succeeded but no outputs returned")
			}
			url := status.Data.Outputs[0].FileURL
			if url == "" {
				return nil, fmt.Errorf("runninghub: empty output URL")
			}
			return &VideoClip{ClipURL: url, DurationSec: 5, ModelUsed: g.Name()}, nil
		case "FAILED":
			return nil, fmt.Errorf("runninghub: task %s failed", taskID)
		}
		// RUNNING — keep polling
	}
}

func (g *RunningHubGenerator) fetchStatus(ctx context.Context, taskID string) (*rhStatusResp, error) {
	url := fmt.Sprintf("%s/task/openapi/status/%s?apiKey=%s", g.BaseURL, taskID, g.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result rhStatusResp
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
