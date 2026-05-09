package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/autovideo/pipeline-service/internal/model"
)

// HTTPClient 负责调用各下游微服务
type HTTPClient struct {
	scriptBase    string
	characterBase string
	imageBase     string
	videoBase     string
	token         string
	http          *http.Client
}

// NewHTTPClient —— 创建下游微服务 HTTP 客户端，设置各服务地址和内部令牌
func NewHTTPClient(scriptBase, characterBase, imageBase, videoBase, token string) *HTTPClient {
	return &HTTPClient{
		scriptBase:    scriptBase,
		characterBase: characterBase,
		imageBase:     imageBase,
		videoBase:     videoBase,
		token:         token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// get —— 发送 GET 请求到指定 URL，自动解包通用响应并反序列化到 result
func (c *HTTPClient) get(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, string(body))
	}

	if result == nil {
		return nil
	}

	// 解包通用响应 {"code":200,"data":{...}}
	var wrapper struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		// 直接解析
		return json.Unmarshal(body, result)
	}
	if wrapper.Data != nil {
		return json.Unmarshal(wrapper.Data, result)
	}
	return nil
}

// post —— 发送 POST 请求到指定 URL，自动序列化请求体并解包响应到 result
func (c *HTTPClient) post(ctx context.Context, url string, body, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("build POST request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s returned %d: %s", url, resp.StatusCode, string(respBody))
	}

	if result == nil {
		return nil
	}

	var wrapper struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return json.Unmarshal(respBody, result)
	}
	if wrapper.Data != nil {
		return json.Unmarshal(wrapper.Data, result)
	}
	return nil
}

// TriggerScriptAnalyze 调用 script-service 触发分析
func (c *HTTPClient) TriggerScriptAnalyze(ctx context.Context, scriptID int64) error {
	url := fmt.Sprintf("%s/api/v1/scripts/%d/analyze", c.scriptBase, scriptID)
	return c.post(ctx, url, nil, nil)
}

// GetScript 获取剧本详情（含 parse_status）
func (c *HTTPClient) GetScript(ctx context.Context, scriptID int64) (*model.ScriptDetail, error) {
	url := fmt.Sprintf("%s/api/v1/scripts/%d", c.scriptBase, scriptID)
	var detail model.ScriptDetail
	if err := c.get(ctx, url, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetScriptCharacters 获取剧本中提取的角色列表
func (c *HTTPClient) GetScriptCharacters(ctx context.Context, scriptID int64) ([]model.Character, error) {
	url := fmt.Sprintf("%s/api/v1/scripts/%d/characters", c.scriptBase, scriptID)
	var chars []model.Character
	if err := c.get(ctx, url, &chars); err != nil {
		return nil, err
	}
	return chars, nil
}

// GetScriptScenes 获取剧本中的场景列表
func (c *HTTPClient) GetScriptScenes(ctx context.Context, scriptID int64) ([]model.Scene, error) {
	url := fmt.Sprintf("%s/api/v1/scripts/%d/scenes", c.scriptBase, scriptID)
	var scenes []model.Scene
	if err := c.get(ctx, url, &scenes); err != nil {
		return nil, err
	}
	return scenes, nil
}

// GetScene 获取单个场景详情
func (c *HTTPClient) GetScene(ctx context.Context, sceneID int64) (*model.Scene, error) {
	url := fmt.Sprintf("%s/api/v1/scenes/%d", c.scriptBase, sceneID)
	var scene model.Scene
	if err := c.get(ctx, url, &scene); err != nil {
		return nil, err
	}
	return &scene, nil
}

// CreateCharacter 在 character-service 创建角色
func (c *HTTPClient) CreateCharacter(ctx context.Context, req model.CreateCharacterReq) (int64, error) {
	url := fmt.Sprintf("%s/api/v1/characters", c.characterBase)
	var result struct {
		ID int64 `json:"id"`
	}
	if err := c.post(ctx, url, req, &result); err != nil {
		return 0, err
	}
	return result.ID, nil
}

// CreateImageTask 向 image-service 提交图片生成任务
func (c *HTTPClient) CreateImageTask(ctx context.Context, req model.CreateImageReq) (int64, error) {
	url := fmt.Sprintf("%s/api/v1/images/generate", c.imageBase)
	var result struct {
		TaskID int64 `json:"task_id"`
	}
	if err := c.post(ctx, url, req, &result); err != nil {
		return 0, err
	}
	return result.TaskID, nil
}

// GetImageTask 查询图片任务状态
func (c *HTTPClient) GetImageTask(ctx context.Context, taskID int64) (*model.ImageTask, error) {
	url := fmt.Sprintf("%s/api/v1/images/tasks/%d", c.imageBase, taskID)
	var task model.ImageTask
	if err := c.get(ctx, url, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// RetryImageTask 重试失败的图片任务
func (c *HTTPClient) RetryImageTask(ctx context.Context, taskID int64) error {
	url := fmt.Sprintf("%s/api/v1/images/tasks/%d/retry", c.imageBase, taskID)
	return c.post(ctx, url, nil, nil)
}

// CreateVideoTask 向 video-service 提交视频生成任务
func (c *HTTPClient) CreateVideoTask(ctx context.Context, req model.CreateVideoReq) (int64, error) {
	url := fmt.Sprintf("%s/api/v1/videos/generate", c.videoBase)
	var result struct {
		TaskID int64 `json:"task_id"`
	}
	if err := c.post(ctx, url, req, &result); err != nil {
		return 0, err
	}
	return result.TaskID, nil
}

// GetVideoTask 查询视频任务状态
func (c *HTTPClient) GetVideoTask(ctx context.Context, taskID int64) (*model.VideoTask, error) {
	url := fmt.Sprintf("%s/api/v1/videos/tasks/%d", c.videoBase, taskID)
	var task model.VideoTask
	if err := c.get(ctx, url, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ComposeVideo 触发视频合成
func (c *HTTPClient) ComposeVideo(ctx context.Context, taskID int64) error {
	url := fmt.Sprintf("%s/api/v1/videos/tasks/%d/compose", c.videoBase, taskID)
	return c.post(ctx, url, nil, nil)
}
