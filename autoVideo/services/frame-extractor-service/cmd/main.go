package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	if err := mergeOverrideConfig(); err != nil {
		return nil, err
	}

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

func mergeOverrideConfig() error {
	overrideFile := strings.TrimSpace(os.Getenv("AUTOVIDEO_CONFIG_OVERRIDE_FILE"))
	if overrideFile == "" {
		return nil
	}
	file, err := os.Open(overrideFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	return viper.MergeConfig(file)
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
			FrameCount int   `json:"frame_count"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		frameCount := req.FrameCount
		if frameCount <= 0 {
			frameCount = 4
		}

		frameURLs, err := extractRepresentativeFrames(ctx, req.VideoURL, req.ProjectID, req.UserID, frameCount, cfg.StorageBaseURL, cfg.StorageBucket, log)
		if err != nil {
			log.Error("frame extraction failed",
				zap.String("video_url", req.VideoURL),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		resp := gin.H{"frame_urls": frameURLs}
		if len(frameURLs) > 0 {
			resp["frame_url"] = frameURLs[0]
		}
		c.JSON(http.StatusOK, resp)
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

// extractRepresentativeFrame downloads the video, extracts a representative frame with ffmpeg,
// uploads it to storage-service, and returns the public URL.
func extractRepresentativeFrame(
	ctx context.Context,
	videoURL string,
	projectID, userID uint64,
	storageBaseURL, bucket string,
	log *zap.Logger,
) (string, error) {
	frameURLs, err := extractRepresentativeFrames(ctx, videoURL, projectID, userID, 1, storageBaseURL, bucket, log)
	if err != nil {
		return "", err
	}
	return frameURLs[0], nil
}

// extractRepresentativeFrames downloads the video, extracts several representative frames,
// uploads them to storage-service, and returns the public URLs in video order.
func extractRepresentativeFrames(
	ctx context.Context,
	videoURL string,
	projectID, userID uint64,
	frameCount int,
	storageBaseURL, bucket string,
	log *zap.Logger,
) ([]string, error) {
	if frameCount < 1 {
		frameCount = 1
	}
	if frameCount > 5 {
		frameCount = 5
	}

	tmpDir, err := os.MkdirTemp("", "frame-extractor-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	videoPath := filepath.Join(tmpDir, "input.mp4")
	if err := downloadFile(ctx, videoURL, videoPath); err != nil {
		return nil, fmt.Errorf("download video: %w", err)
	}

	duration, err := getVideoDuration(ctx, videoPath)
	if err != nil {
		log.Warn("could not get video duration, sampling from early timestamps", zap.Error(err))
		duration = -1
	}

	timestamps := buildFrameTimestamps(duration, frameCount)
	frameURLs := make([]string, 0, len(timestamps))

	for index, timestamp := range timestamps {
		framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%02d.jpg", index+1))
		if err := extractFrameAtTime(ctx, videoPath, timestamp, framePath); err != nil {
			log.Warn("frame extraction skipped",
				zap.String("video_url", videoURL),
				zap.Int("frame_index", index+1),
				zap.Float64("timestamp_sec", timestamp),
				zap.Error(err),
			)
			continue
		}

		if _, err := os.Stat(framePath); err != nil {
			log.Warn("frame file not created",
				zap.String("video_url", videoURL),
				zap.Int("frame_index", index+1),
				zap.Float64("timestamp_sec", timestamp),
				zap.Error(err),
			)
			continue
		}

		frameURL, err := uploadFrameToStorage(ctx, framePath, projectID, userID, storageBaseURL, bucket)
		if err != nil {
			log.Warn("frame upload skipped",
				zap.String("video_url", videoURL),
				zap.Int("frame_index", index+1),
				zap.Float64("timestamp_sec", timestamp),
				zap.Error(err),
			)
			continue
		}

		frameURLs = append(frameURLs, frameURL)
	}

	if len(frameURLs) == 0 {
		return nil, fmt.Errorf("no frame urls extracted")
	}

	log.Info("representative frames extracted",
		zap.String("video_url", videoURL),
		zap.Int("frame_count", len(frameURLs)),
	)
	return frameURLs, nil
}

func extractFrameAtTime(ctx context.Context, videoPath string, timestamp float64, framePath string) error {
	args := []string{
		"-ss", fmt.Sprintf("%.3f", timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		framePath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract frame: %w; stderr: %s", err, stderr.String())
	}
	return nil
}

func buildFrameTimestamps(duration float64, frameCount int) []float64 {
	if frameCount <= 1 {
		if duration > 0 {
			return []float64{duration * 0.5}
		}
		return []float64{1}
	}

	timestamps := make([]float64, 0, frameCount)
	if duration <= 0 {
		for index := 0; index < frameCount; index++ {
			timestamps = append(timestamps, 1+float64(index*2))
		}
		return timestamps
	}

	startFraction := 0.15
	endFraction := 0.85
	step := (endFraction - startFraction) / float64(frameCount-1)
	for index := 0; index < frameCount; index++ {
		fraction := startFraction + step*float64(index)
		timestamps = append(timestamps, duration*fraction)
	}
	return timestamps
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

	var parsed struct {
		URL string `json:"url"`
		Data struct {
			URL     string `json:"url"`
			CDNURL  string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse storage response: %w", err)
	}

	if parsed.URL != "" {
		return parsed.URL, nil
	}
	if parsed.Data.CDNURL != "" {
		return parsed.Data.CDNURL, nil
	}
	if parsed.Data.URL != "" {
		return parsed.Data.URL, nil
	}

	return "", fmt.Errorf("no url in storage response: %s", string(body))
}
