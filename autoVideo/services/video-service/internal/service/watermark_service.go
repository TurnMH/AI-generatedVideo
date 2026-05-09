package service

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// WatermarkConfig mirrors the frontend watermark configuration type.
type WatermarkConfig struct {
	Enabled  bool     `json:"enabled"`
	Type     string   `json:"type"`               // text or image
	Text     string   `json:"text,omitempty"`      // used when Type == "text"
	ImageURL string   `json:"image_url,omitempty"` // used when Type == "image"
	Position string   `json:"position"`            // top-left, top-right, bottom-left, bottom-right, center
	Opacity  float64  `json:"opacity"`
	Size     string   `json:"size"` // small, medium, large
	ApplyTo  []string `json:"apply_to"`
}

// WatermarkService handles watermark overlay operations.
type WatermarkService struct {
	ffmpeg *FFmpegService
	logger *zap.Logger
}

// NewWatermarkService —— 创建水印服务实例
// NewWatermarkService creates a new WatermarkService.
func NewWatermarkService(ffmpeg *FFmpegService, logger *zap.Logger) *WatermarkService {
	return &WatermarkService{ffmpeg: ffmpeg, logger: logger}
}

// ApplyWatermark —— 生成水印叠加的 FFmpeg 命令（当前为 dry-run 模式）
// ApplyWatermark generates an FFmpeg command to overlay a watermark on inputPath
// and writes the result to outputPath. Currently logs the command without executing
// since ffmpeg may not be available in all environments.
func (s *WatermarkService) ApplyWatermark(inputPath, outputPath string, config WatermarkConfig) error {
	if !config.Enabled {
		return nil
	}

	filter, err := s.buildFilter(config)
	if err != nil {
		return err
	}

	args := []string{"-i", inputPath}

	if config.Type == "image" && config.ImageURL != "" {
		args = append(args, "-i", config.ImageURL)
	}

	args = append(args, "-filter_complex", filter, "-c:a", "copy", outputPath)

	s.logger.Info("applying watermark",
		zap.Strings("args", args),
		zap.String("input", inputPath),
		zap.String("output", outputPath),
	)

	return s.ffmpeg.RunFFmpeg(context.Background(), args...)
}

// buildFilter —— 根据水印配置构建 FFmpeg filter_complex 字符串
// buildFilter constructs the FFmpeg filter_complex string for the watermark.
func (s *WatermarkService) buildFilter(cfg WatermarkConfig) (string, error) {
	opacity := cfg.Opacity
	if opacity <= 0 || opacity > 1 {
		opacity = 0.5
	}

	x, y := positionToXY(cfg.Position)
	fontSize := sizeToFontSize(cfg.Size)

	switch cfg.Type {
	case "text":
		if cfg.Text == "" {
			return "", fmt.Errorf("watermark text is empty")
		}
		escaped := strings.ReplaceAll(cfg.Text, "'", "'\\''")
		filter := fmt.Sprintf(
			"drawtext=text='%s':fontsize=%d:fontcolor=white@%.2f:x=%s:y=%s",
			escaped, fontSize, opacity, x, y,
		)
		return filter, nil

	case "image":
		if cfg.ImageURL == "" {
			return "", fmt.Errorf("watermark image_url is empty")
		}
		scale := sizeToScale(cfg.Size)
		filter := fmt.Sprintf(
			"[1:v]scale=%s,format=rgba,colorchannelmixer=aa=%.2f[wm];[0:v][wm]overlay=%s:%s",
			scale, opacity, x, y,
		)
		return filter, nil

	default:
		return "", fmt.Errorf("unsupported watermark type: %s", cfg.Type)
	}
}

// positionToXY —— 将位置名称映射为 FFmpeg overlay 坐标
// positionToXY maps a named position to FFmpeg overlay coordinates.
func positionToXY(position string) (string, string) {
	switch position {
	case "top-left":
		return "10", "10"
	case "top-right":
		return "W-w-10", "10"
	case "bottom-left":
		return "10", "H-h-10"
	case "bottom-right":
		return "W-w-10", "H-h-10"
	case "center":
		return "(W-w)/2", "(H-h)/2"
	default:
		return "10", "10"
	}
}

// sizeToFontSize —— 将尺寸标签映射为 drawtext 字体大小
// sizeToFontSize maps size labels to drawtext font sizes.
func sizeToFontSize(size string) int {
	switch size {
	case "small":
		return 18
	case "large":
		return 48
	default: // medium
		return 30
	}
}

// sizeToScale —— 将尺寸标签映射为图片水印的 FFmpeg scale 值
// sizeToScale maps size labels to FFmpeg scale values for image watermarks.
func sizeToScale(size string) string {
	switch size {
	case "small":
		return "iw/6:-1"
	case "large":
		return "iw/2:-1"
	default: // medium
		return "iw/4:-1"
	}
}
