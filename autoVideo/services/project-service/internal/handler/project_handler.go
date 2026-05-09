package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autovideo/project-service/internal/service"
	"github.com/autovideo/project-service/pkg/middleware"
	"github.com/autovideo/project-service/pkg/response"
	"github.com/autovideo/project-service/pkg/textdecode"
)

type ProjectHandler struct {
	svc              *service.ProjectService
	storageBaseURL   string
	characterBaseURL string
	imageBaseURL     string
	videoBaseURL     string
	scriptBaseURL    string
	jwtSecret        string
}

// NewProjectHandler —— 创建项目处理器实例
func NewProjectHandler(svc *service.ProjectService, storageBaseURL, characterBaseURL, imageBaseURL, videoBaseURL, scriptBaseURL, jwtSecret string) *ProjectHandler {
	if storageBaseURL == "" {
		storageBaseURL = "http://localhost:8009"
	}
	if characterBaseURL == "" {
		characterBaseURL = "http://localhost:8004"
	}
	if imageBaseURL == "" {
		imageBaseURL = "http://localhost:8005"
	}
	if videoBaseURL == "" {
		videoBaseURL = "http://localhost:8006"
	}
	if scriptBaseURL == "" {
		scriptBaseURL = "http://localhost:8003"
	}
	return &ProjectHandler{
		svc:              svc,
		storageBaseURL:   storageBaseURL,
		characterBaseURL: characterBaseURL,
		imageBaseURL:     imageBaseURL,
		videoBaseURL:     videoBaseURL,
		scriptBaseURL:    scriptBaseURL,
		jwtSecret:        jwtSecret,
	}
}

// buildServiceToken creates a short-lived JWT with role=service for internal calls.
func (h *ProjectHandler) buildServiceToken() string {
	if h.jwtSecret == "" {
		return ""
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(map[string]interface{}{
		"user_id":    1,
		"project_id": 0,
		"role":       "service",
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// List —— 处理分页查询项目列表的请求
// List godoc
// GET /api/v1/projects?keyword=&status=&page=1&page_size=20
func (h *ProjectHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	req := service.ListProjectsReq{
		UserID:      userID,
		Keyword:     c.Query("keyword"),
		Status:      c.Query("status"),
		ProjectType: c.Query("project_type"),
		Page:        page,
		PageSize:    pageSize,
	}

	projects, total, err := h.svc.List(req)
	if err != nil {
		response.InternalError(c, "failed to list projects: "+err.Error())
		return
	}

	response.OKList(c, projects, page, pageSize, total)
}

// Create —— 处理创建新项目的请求
// Create godoc
// POST /api/v1/projects
func (h *ProjectHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req service.CreateProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.UserID = userID

	project, err := h.svc.Create(req)
	if err != nil {
		response.InternalError(c, "failed to create project: "+err.Error())
		return
	}

	// Async: seed default skills for the new project in character-service
	go func() {
		// Extract style_preset from storyboard_config to differentiate live-action vs animation skills
		stylePreset := ""
		if req.StoryboardConfig != nil {
			if sp, ok := req.StoryboardConfig["style_preset"].(string); ok {
				stylePreset = sp
			}
		}
		body, _ := json.Marshal(map[string]interface{}{
			"project_id":   int64(project.ID),
			"style_preset": stylePreset,
		})
		url := h.characterBaseURL + "/internal/seed-skills"
		httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Printf("[WARN] seed-skills build request failed for project %d: %v", project.ID, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if token := h.buildServiceToken(); token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
		if err != nil {
			log.Printf("[WARN] seed-skills request failed for project %d: %v", project.ID, err)
			return
		}
		io.ReadAll(resp.Body) //nolint:errcheck // drain body to allow connection reuse
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("[WARN] seed-skills returned HTTP %d for project %d", resp.StatusCode, project.ID)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "created",
		"data":    project,
	})
}

// Get —— 处理获取单个项目详情的请求
// Get godoc
// GET /api/v1/projects/:id
func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var project interface{}
	// Service-to-service calls use role="service" and must not be blocked by user ownership.
	if middleware.GetRole(c) == "service" {
		project, err = h.svc.GetNoAuth(id)
	} else {
		project, err = h.svc.Get(id, middleware.GetUserID(c))
	}
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, project)
}

// GetProgress —— 处理获取项目进度的轻量级轮询请求
// GetProgress returns only the progress JSON for a project (lightweight polling endpoint).
// GET /api/v1/projects/:id/progress
func (h *ProjectHandler) GetProgress(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	project, err := h.svc.Get(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"status":   project.Status,
		"progress": project.Progress,
	})
}

// Update —— 处理更新项目信息的请求
// Update godoc
// PUT /api/v1/projects/:id
func (h *ProjectHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req service.UpdateProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	project, err := h.svc.Update(id, userID, req)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, project)
}

// Delete —— 处理删除项目的请求
// Delete godoc
// DELETE /api/v1/projects/:id
func (h *ProjectHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	if _, err := h.svc.Get(id, userID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	if err := h.cleanupProjectResources(id, userID, c.GetHeader("Authorization")); err != nil {
		response.InternalError(c, "cleanup project resources failed: "+err.Error())
		return
	}

	if err := h.svc.Delete(id, userID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

func (h *ProjectHandler) cleanupProjectResources(projectID, userID uint64, authHeader string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	cleanupTargets := []string{
		// character-service: delete all assets (incl. locked), characters, skills, production skills
		fmt.Sprintf("%s/api/v1/projects/%d/runtime-data", h.characterBaseURL, projectID),
		fmt.Sprintf("%s/api/v1/projects/%d/images/runtime-data", h.imageBaseURL, projectID),
		fmt.Sprintf("%s/api/v1/projects/%d/videos/runtime-data", h.videoBaseURL, projectID),
		fmt.Sprintf("%s/api/v1/projects/%d/dubbing/runtime-data", h.videoBaseURL, projectID),
		// script-service: delete all scripts + scenes/characters/assets/split-config
		fmt.Sprintf("%s/api/v1/projects/%d/runtime-data", h.scriptBaseURL, projectID),
		fmt.Sprintf("%s/api/v1/projects/%d/storage", h.storageBaseURL, projectID),
	}

	for _, endpoint := range cleanupTargets {
		req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
		if err != nil {
			log.Printf("[WARN] cleanupProjectResources: build request failed for %s: %v", endpoint, err)
			continue
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

		resp, err := client.Do(req)
		if err != nil {
			// Service unreachable (not running / network error) — log and continue
			log.Printf("[WARN] cleanupProjectResources: %s unreachable: %v", endpoint, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// 404 means the resource never existed or is already gone — treat as success
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			// Log but don't block project deletion
			log.Printf("[WARN] cleanupProjectResources: %s returned %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}

	return nil
}

// Pause —— 处理暂停项目的请求
// Pause godoc
// POST /api/v1/projects/:id/pause
func (h *ProjectHandler) Pause(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	if err := h.svc.Pause(id, userID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OK(c, gin.H{"status": "paused"})
}

// Resume —— 处理恢复已暂停项目的请求
// Resume godoc
// POST /api/v1/projects/:id/resume
func (h *ProjectHandler) Resume(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	if err := h.svc.Resume(id, userID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OK(c, gin.H{"status": "resumed"})
}

// Clone —— 处理克隆项目的请求，返回新项目
// Clone godoc
// POST /api/v1/projects/:id/clone
func (h *ProjectHandler) Clone(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	project, err := h.svc.Clone(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "cloned",
		"data":    project,
	})
}

// GetScriptVersions —— 处理获取项目剧本版本列表的请求
// GetScriptVersions godoc
// GET /api/v1/projects/:id/script/versions
func (h *ProjectHandler) GetScriptVersions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	versions, err := h.svc.GetScriptVersions(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, versions)
}

// SwitchScriptVersion —— 处理切换当前剧本版本的请求
// SwitchScriptVersion godoc
// POST /api/v1/projects/:id/script/switch/:versionId
func (h *ProjectHandler) SwitchScriptVersion(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	versionID, err := parseUint64Param(c, "versionId")
	if err != nil {
		response.BadRequest(c, "invalid version id")
		return
	}

	if err := h.svc.SwitchScriptVersion(id, userID, versionID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "script version switched"})
}

// UploadScript —— 处理上传剧本文件的请求，保存文件并创建版本记录
// UploadScript godoc
// POST /api/v1/projects/:id/script
func (h *ProjectHandler) UploadScript(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file field required: "+err.Error())
		return
	}

	// Limit to 20MB for script files
	const maxSize = 20 << 20
	if fh.Size > maxSize {
		response.BadRequest(c, "file too large, max 20MB")
		return
	}

	f, err := fh.Open()
	if err != nil {
		response.InternalError(c, "cannot open file")
		return
	}
	defer f.Close()

	// Read file content for script_text storage
	fileBytes, err := io.ReadAll(f)
	if err != nil {
		response.InternalError(c, "read file failed")
		return
	}
	scriptText, err := textdecode.Decode(fileBytes)
	if err != nil {
		response.BadRequest(c, "script file encoding is not supported")
		return
	}

	// Reset reader for upload
	reader := bytes.NewReader(fileBytes)

	// Forward file to storage-service
	fileURL, err := h.uploadToStorage(id, userID, fmt.Sprintf("scripts/%s", fh.Filename), "script", reader, fh)
	if err != nil {
		response.InternalError(c, "upload to storage failed: "+err.Error())
		return
	}

	// Save script URL, text content, and create version
	if err := h.svc.UploadScript(id, userID, fileURL, fh.Filename, fh.Size, scriptText); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"message":          "script uploaded",
		"script_file_url":  fileURL,
		"script_file_size": fh.Size,
	})
}

// uploadToStorage —— 通过 multipart HTTP 将文件代理上传到 storage-service，返回 CDN URL
// uploadToStorage proxies a file to storage-service via multipart HTTP.
func (h *ProjectHandler) uploadToStorage(projectID, userID uint64, filename, category string, reader io.Reader, fh *multipart.FileHeader) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	_ = mw.WriteField("bucket", "scripts")
	_ = mw.WriteField("user_id", fmt.Sprintf("%d", userID))
	_ = mw.WriteField("project_id", fmt.Sprintf("%d", projectID))
	_ = mw.WriteField("category", category)

	fw, err := mw.CreateFormFile("file", fh.Filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, reader); err != nil {
		return "", err
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, h.storageBaseURL+"/api/v1/storage/upload", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage-service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("storage-service error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response: {"code":0,"message":"ok","data":{"object_key":"...","cdn_url":"...","file_id":N}}
	var result struct {
		Code int `json:"code"`
		Data struct {
			CdnURL    string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse storage response: %w", err)
	}
	if result.Data.CdnURL == "" {
		return "", fmt.Errorf("storage-service returned no URL")
	}
	return result.Data.CdnURL, nil
}

// CreateSnapshot —— 处理创建项目快照的请求
// CreateSnapshot godoc
// POST /api/v1/projects/:id/snapshot
func (h *ProjectHandler) CreateSnapshot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	snapshot, err := h.svc.CreateSnapshot(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "snapshot created",
		"data":    snapshot,
	})
}

// ListSnapshots —— 处理获取项目快照列表的请求
// ListSnapshots godoc
// GET /api/v1/projects/:id/snapshots
func (h *ProjectHandler) ListSnapshots(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	snapshots, err := h.svc.ListSnapshots(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, snapshots)
}

// parseUint64Param —— 从 URL 参数中解析 uint64 类型的值
func parseUint64Param(c *gin.Context, key string) (uint64, error) {
	v, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// isNotFound —— 判断错误是否为未找到类型
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "project not found" || msg == "episode not found"
}
