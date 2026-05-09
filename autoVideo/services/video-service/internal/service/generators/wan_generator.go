package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"
)

// WanGenerator wraps the DashScope Wanx video generation API (image→video).
type WanGenerator struct {
	APIKey  string
	BaseURL string
	client  *http.Client
}

// NewWanGenerator —— 创建通义万相视频生成器实例
func NewWanGenerator(apiKey, _ /* secretKey unused */, baseURL string) *WanGenerator {
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com"
	}
	return &WanGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称 "wan"
func (g *WanGenerator) Name() string { return "wan" }

// IsAvailable —— 检查 API Key 是否已配置
func (g *WanGenerator) IsAvailable(ctx context.Context) bool {
	return g.APIKey != ""
}

// SupportsNativeAudio —— Wan does not embed audio in generated clips.
func (g *WanGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— Wan 支持的模型参数（时长固定，支持宽高比选择）
func (g *WanGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "5", Label: "5秒（固定）"}},
		},
		{
			Key: "aspect_ratio", Label: "画面比例", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"},
			},
		},
	}
}
type wanSubmitReq struct {
	Model      string            `json:"model"`
	Input      wanSubmitInput    `json:"input"`
	Parameters map[string]any    `json:"parameters"`
}

type wanSubmitInput struct {
	ImgURL       string   `json:"img_url,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
	Function     string   `json:"function"`
	ExtendPrompt bool     `json:"extend_prompt"`
	RefImgs      []string `json:"ref_imgs,omitempty"` // character reference images for consistency
}

type wanSubmitResp struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type wanPollResp struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"` // PENDING, RUNNING, SUCCEEDED, FAILED
		VideoURL   string `json:"video_url"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DashScope OSS upload types
type ossPolicy struct {
	Data struct {
		Policy           string `json:"policy"`
		Signature        string `json:"signature"`
		UploadDir        string `json:"upload_dir"`
		UploadHost       string `json:"upload_host"`
		OSSAccessKeyID   string `json:"oss_access_key_id"`
		XOSSObjectACL    string `json:"x_oss_object_acl"`
		XOSSForbidOW     string `json:"x_oss_forbid_overwrite"`
	} `json:"data"`
}

// wanAspectRatioParams maps a human-readable aspect ratio to the DashScope `size` param.
// DashScope wanx2.1-i2v accepts: "1280*720" (16:9), "720*1280" (9:16), "720*720" (1:1).
func wanAspectRatioParams(aspectRatio string) map[string]any {
	sizeMap := map[string]string{
		"16:9": "1280*720",
		"9:16": "720*1280",
		"1:1":  "720*720",
	}
	params := map[string]any{}
	if size, ok := sizeMap[aspectRatio]; ok {
		params["size"] = size
	}
	return params
}

// Generate —— 提交图生视频任务并轮询等待完成，返回 *VideoClip
func (g *WanGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("wan submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("wan poll: %w", err)
	}
	return clip, nil
}

// submit —— 向 DashScope API 提交视频生成请求，返回任务 ID
func (g *WanGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	imgURL := req.SourceImageURL
	needOSSResolve := false

	// If the image is on a local/internal URL, upload to DashScope OSS first
	if isLocalURL(imgURL) {
		ossURL, err := g.uploadToOSS(ctx, imgURL)
		if err != nil {
			return "", fmt.Errorf("upload to OSS: %w", err)
		}
		imgURL = ossURL
		needOSSResolve = true
	}

	body := wanSubmitReq{
		Model: "wanx2.1-i2v-turbo",
		Input: wanSubmitInput{
			ImgURL:       imgURL,
			Prompt:       req.Prompt,
			Function:     "video-synthesis",
			ExtendPrompt: true,
			RefImgs:      req.CharacterImageURLs, // inject character reference images
		},
		Parameters: wanAspectRatioParams(req.AspectRatio),
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/api/v1/services/aigc/video-generation/video-synthesis", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)
	httpReq.Header.Set("X-DashScope-Async", "enable")
	if needOSSResolve {
		httpReq.Header.Set("X-DashScope-OssResourceResolve", "enable")
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("wan rate limited (429)")
	}

	var result wanSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Code != "" {
		return "", fmt.Errorf("wan error %s: %s", result.Code, result.Message)
	}
	if result.Output.TaskID == "" {
		return "", fmt.Errorf("wan: no task_id in response: %s", string(respBody))
	}
	return result.Output.TaskID, nil
}

// poll —— 轮询 DashScope 任务状态直到完成、失败或超时（15分钟）
func (g *WanGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("wan: task %s timed out after 15min", taskID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, taskID)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

// queryTask —— 查询 DashScope 任务状态，返回结果和是否完成
func (g *WanGenerator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/api/v1/tasks/"+taskID, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil // transient error, retry
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result wanPollResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil
	}

	switch result.Output.TaskStatus {
	case "SUCCEEDED":
		return &VideoClip{
			ClipURL:     result.Output.VideoURL,
			DurationSec: 5,
			ModelUsed:   "wan",
		}, true, nil
	case "FAILED":
		msg := result.Message
		if msg == "" {
			msg = "task failed"
		}
		return nil, false, fmt.Errorf("wan: %s", msg)
	default: // PENDING, RUNNING
		return nil, false, nil
	}
}

// isLocalURL —— 判断 URL 是否指向本地或内网地址
// isLocalURL returns true if the URL points to a local or internal address
// that DashScope servers cannot access.
func isLocalURL(u string) bool {
	return strings.Contains(u, "localhost") ||
		strings.Contains(u, "127.0.0.1") ||
		strings.Contains(u, "0.0.0.0") ||
		strings.HasPrefix(u, "http://10.") ||
		strings.HasPrefix(u, "http://192.168.")
}

// uploadToOSS —— 将本地 URL 的图片上传到 DashScope OSS，返回 oss:// 地址
// uploadToOSS downloads an image from a local URL, uploads it to DashScope's
// OSS via the getPolicy mechanism, and returns an oss:// URL usable in API calls.
func (g *WanGenerator) uploadToOSS(ctx context.Context, srcURL string) (string, error) {
	// 1. Download image from local URL
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", fmt.Errorf("build download req: %w", err)
	}
	dlResp, err := g.client.Do(dlReq)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != 200 {
		return "", fmt.Errorf("download image: HTTP %d", dlResp.StatusCode)
	}
	imgBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	// 2. Get upload policy from DashScope
	policyURL := g.BaseURL + "/api/v1/uploads?action=getPolicy&model=wanx2.1-i2v-turbo"
	pReq, err := http.NewRequestWithContext(ctx, http.MethodGet, policyURL, nil)
	if err != nil {
		return "", err
	}
	pReq.Header.Set("Authorization", "Bearer "+g.APIKey)
	pResp, err := g.client.Do(pReq)
	if err != nil {
		return "", fmt.Errorf("get upload policy: %w", err)
	}
	defer pResp.Body.Close()

	var policy ossPolicy
	if err := json.NewDecoder(pResp.Body).Decode(&policy); err != nil {
		return "", fmt.Errorf("parse upload policy: %w", err)
	}

	// 3. Upload to OSS
	filename := path.Base(srcURL)
	fileKey := policy.Data.UploadDir + "/" + filename

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("OSSAccessKeyId", policy.Data.OSSAccessKeyID)
	w.WriteField("policy", policy.Data.Policy)
	w.WriteField("Signature", policy.Data.Signature)
	w.WriteField("key", fileKey)
	w.WriteField("x-oss-object-acl", policy.Data.XOSSObjectACL)
	w.WriteField("x-oss-forbid-overwrite", policy.Data.XOSSForbidOW)
	w.WriteField("success_action_status", "200")
	contentType := mime.TypeByExtension(path.Ext(filename))
	if contentType == "" {
		contentType = "image/png"
	}
	w.WriteField("x-oss-content-type", contentType)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	fw.Write(imgBytes)
	w.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.Data.UploadHost, &buf)
	if err != nil {
		return "", err
	}
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())

	uploadResp, err := g.client.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("OSS upload: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != 200 {
		body, _ := io.ReadAll(uploadResp.Body)
		return "", fmt.Errorf("OSS upload: HTTP %d: %s", uploadResp.StatusCode, string(body))
	}

	// 4. Return the oss:// URL — DashScope SDK format is oss://<key> (no bucket prefix)
	ossURL := fmt.Sprintf("oss://%s", fileKey)
	return ossURL, nil
}
