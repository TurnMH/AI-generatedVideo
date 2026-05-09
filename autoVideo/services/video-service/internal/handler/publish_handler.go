package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/autovideo/video-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PublishHandler handles video publishing to social media platforms (feat-11).
// Currently supported: Douyin (TikTok CN), WeChat Channels via Upload-Post API.
type PublishHandler struct {
	uploadPostKey string // Upload-Post API key (https://upload-post.com)
	uploadPostURL string
	client        *http.Client
	logger        *zap.Logger
}

// NewPublishHandler creates a publish handler. When uploadPostKey is empty the
// handler returns an informative error on all requests.
func NewPublishHandler(uploadPostKey, uploadPostURL string, logger *zap.Logger) *PublishHandler {
	if uploadPostURL == "" {
		uploadPostURL = "https://api.upload-post.com/api"
	}
	return &PublishHandler{
		uploadPostKey: uploadPostKey,
		uploadPostURL: uploadPostURL,
		client:        &http.Client{Timeout: 60 * time.Second},
		logger:        logger,
	}
}

// publishReq is the request body for POST /api/v1/projects/:pid/videos/:vid/publish
type publishReq struct {
	// Platform: "douyin" | "wechat_channels" | "tiktok" | "instagram"
	Platform string `json:"platform" binding:"required"`
	// AccountID is the platform account identifier registered in Upload-Post
	AccountID string `json:"account_id" binding:"required"`
	// Caption is the post title/caption
	Caption string `json:"caption"`
	// HashTags is a list of hashtag strings (without #)
	HashTags []string `json:"hashtags"`
}

// uploadPostRequest is the Upload-Post API payload
type uploadPostRequest struct {
	AccountID string   `json:"account_id"`
	VideoURL  string   `json:"video_url"`
	Caption   string   `json:"caption"`
	HashTags  []string `json:"hashtags,omitempty"`
	Platform  string   `json:"platform"`
}

type uploadPostResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	PostID  string `json:"post_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PublishVideo godoc
// POST /api/v1/projects/:pid/videos/:vid/publish
// Publishes a completed video to the specified social media platform.
func (h *PublishHandler) PublishVideo(c *gin.Context) {
	if h.uploadPostKey == "" {
		response.Fail(c, http.StatusServiceUnavailable, 503, "social publish not configured: set models.upload_post_key in config")
		return
	}

	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	var req publishReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// We need the video URL — the caller should pass it, OR we can look it up.
	// For simplicity, accept video_url as optional override; otherwise require it.
	videoURL := c.Query("video_url")
	if videoURL == "" {
		response.BadRequest(c, "video_url query parameter is required (the public URL of the completed video)")
		return
	}

	postID, err := h.publish(c.Request.Context(), req, videoURL)
	if err != nil {
		h.logger.Error("social publish failed",
			zap.Int64("video_task_id", vid),
			zap.String("platform", req.Platform),
			zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	h.logger.Info("social publish submitted",
		zap.Int64("video_task_id", vid),
		zap.String("platform", req.Platform),
		zap.String("post_id", postID))

	response.OK(c, gin.H{
		"post_id":  postID,
		"platform": req.Platform,
		"status":   "submitted",
	})
}

func (h *PublishHandler) publish(ctx context.Context, req publishReq, videoURL string) (string, error) {
	payload := uploadPostRequest{
		AccountID: req.AccountID,
		VideoURL:  videoURL,
		Caption:   req.Caption,
		HashTags:  req.HashTags,
		Platform:  req.Platform,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal publish request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.uploadPostURL+"/publish", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.uploadPostKey)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("upload-post API request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result uploadPostResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse upload-post response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("upload-post publish failed: %s", result.Error)
	}
	return result.PostID, nil
}
