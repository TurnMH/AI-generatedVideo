package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config holds all service configuration.
type Config struct {
	Port           int    `mapstructure:"port"`
	StorageBaseURL string `mapstructure:"storage_base_url"` // http://localhost:8009
	StorageBucket  string `mapstructure:"storage_bucket"`   // e.g. "j1-common-bucket"
	JWTSecret      string `mapstructure:"jwt_secret"`
}

func loadConfig() (*Config, error) {
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("FRAME_EXTRACTOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("port", 8010)
	viper.SetDefault("storage_base_url", "http://localhost:8009")
	viper.SetDefault("storage_bucket", "j1-common-bucket")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Fall back to the ignored local runtime config in the repo root.
		viper.SetConfigName("config.local")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
	}
	_ = viper.ReadInConfig() // ignore error — env vars are the fallback

	// Map shared config paths
	if v := viper.GetString("storage.base_url"); v != "" {
		viper.SetDefault("storage_base_url", v)
	}
	if v := viper.GetString("jwt.access_secret"); v != "" {
		viper.SetDefault("jwt_secret", v)
	}

	var cfg Config
	cfg.Port = viper.GetInt("port")
	cfg.StorageBaseURL = viper.GetString("storage_base_url")
	cfg.StorageBucket = viper.GetString("storage_bucket")
	cfg.JWTSecret = viper.GetString("jwt_secret")
	return &cfg, nil
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal("load config failed", zap.Error(err))
	}

	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Warn("ffmpeg not found in PATH; frame extraction will fail", zap.Error(err))
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/extract", func(c *gin.Context) {
		var req struct {
			VideoURL  string `json:"video_url" binding:"required"`
			ProjectID uint64 `json:"project_id"`
			UserID    uint64 `json:"user_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		frameURL, err := extractLastFrame(ctx, req.VideoURL, req.ProjectID, req.UserID, cfg.StorageBaseURL, cfg.StorageBucket, log)
		if err != nil {
			log.Error("frame extraction failed",
				zap.String("video_url", req.VideoURL),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"frame_url": frameURL})
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Info("frame-extractor-service starting", zap.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("frame-extractor-service stopped")
}

// extractLastFrame downloads the video, extracts the last frame with ffmpeg,
// uploads it to storage-service, and returns the public URL.
func extractLastFrame(
	ctx context.Context,
	videoURL string,
	projectID, userID uint64,
	storageBaseURL, bucket string,
	log *zap.Logger,
) (string, error) {
	// 1. Create temp directory
	tmpDir, err := os.MkdirTemp("", "frame-extractor-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	videoPath := filepath.Join(tmpDir, "input.mp4")
	framePath := filepath.Join(tmpDir, "last_frame.jpg")

	// 2. Download the video
	if err := downloadFile(ctx, videoURL, videoPath); err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}

	// 3. Get video duration using ffprobe
	duration, err := getVideoDuration(ctx, videoPath)
	if err != nil {
		log.Warn("could not get video duration, seeking from end", zap.Error(err))
		duration = -1
	}

	// 4. Extract last frame using ffmpeg
	// Strategy: seek to near the end (-sseof -0.5 seeks 0.5s from end), grab 1 frame
	var args []string
	if duration > 0 {
		// Seek to 0.5s before the end
		seekTime := duration - 0.5
		if seekTime < 0 {
			seekTime = 0
		}
		args = []string{
			"-ss", fmt.Sprintf("%.3f", seekTime),
			"-i", videoPath,
			"-vframes", "1",
			"-q:v", "2",
			"-y",
			framePath,
		}
	} else {
		// Fallback: use -sseof (seek from end)
		args = []string{
			"-sseof", "-1",
			"-i", videoPath,
			"-vframes", "1",
			"-q:v", "2",
			"-update", "1",
			"-y",
			framePath,
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg extract frame: %w; stderr: %s", err, stderr.String())
	}

	if _, err := os.Stat(framePath); err != nil {
		return "", fmt.Errorf("frame file not created: %w", err)
	}

	// 5. Upload to storage-service
	frameURL, err := uploadFrameToStorage(ctx, framePath, projectID, userID, storageBaseURL, bucket)
	if err != nil {
		return "", fmt.Errorf("upload frame: %w", err)
	}

	log.Info("last frame extracted",
		zap.String("video_url", videoURL),
		zap.String("frame_url", frameURL),
	)
	return frameURL, nil
}

// downloadFile downloads a URL to a local file path.
func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// getVideoDuration uses ffprobe to get the video duration in seconds.
func getVideoDuration(ctx context.Context, videoPath string) (float64, error) {
	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var dur float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &dur)
	return dur, err
}

// uploadFrameToStorage uploads the frame file as multipart/form-data to storage-service.
func uploadFrameToStorage(ctx context.Context, framePath string, projectID, userID uint64, storageBaseURL, bucket string) (string, error) {
	f, err := os.Open(framePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Write form fields
	_ = mw.WriteField("bucket", bucket)
	_ = mw.WriteField("user_id", fmt.Sprintf("%d", userID))
	_ = mw.WriteField("project_id", fmt.Sprintf("%d", projectID))
	_ = mw.WriteField("category", "frame")

	// Write the file
	fw, err := mw.CreateFormFile("file", filepath.Base(framePath))
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(fw, f); err != nil {
		return "", err
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		storageBaseURL+"/api/v1/storage/upload",
		&buf,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("storage upload HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse URL from response: {"data":{"url":"..."}} or {"url":"..."}
	urlStr := extractJSONString(string(body), "url")
	if urlStr == "" {
		return "", fmt.Errorf("no url in storage response: %s", string(body))
	}
	return urlStr, nil
}

// extractJSONString is a minimal JSON string extractor for a single key.
func extractJSONString(jsonStr, key string) string {
	needle := `"` + key + `":"`
	idx := strings.Index(jsonStr, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(jsonStr[start:], `"`)
	if end < 0 {
		return ""
	}
	return jsonStr[start : start+end]
}
