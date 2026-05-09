package service

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"strings"
)

// ─── Vision QA result types ───────────────────────────────────────────────────

// PanelVisionResult holds the AI quality-check outcome for a single panel.
type PanelVisionResult struct {
	Panel  string   `json:"panel"`            // closeup | front | side | back
	Pass   bool     `json:"pass"`             // true = meets standard
	Score  int      `json:"score"`            // 1-10
	Issues []string `json:"issues,omitempty"` // human-readable problem descriptions
}

// PanelVisionReport is the full AI validation report for all 4 panels of a character asset.
type PanelVisionReport struct {
	Panels      []PanelVisionResult `json:"panels"`
	OverallPass bool                `json:"overall_pass"`
	Summary     string              `json:"summary"`
	Model       string              `json:"model"`
	CheckedAt   string              `json:"checked_at"`
}

// panelValidationResult —— 单栏质检结果，fatal=true 表示强烈建议重绘。
type panelValidationResult struct {
	Panel       CharacterPanel
	URL         string
	Width       int
	Height      int
	AspectRatio float64
	// Brightness —— 平均亮度（0..255），过暗/过亮常见于全黑/全白废图。
	Brightness float64
	Issues     []string
	Fatal      bool
}

// validatePanels —— 对 panelURLs 做纯 Go 质检（宽高比 / 亮度直方图），不依赖外部模型。
// 返回每栏结果；失败的下载/解码会被标记为 Fatal=true 并记录 issue。
func validatePanels(ctx context.Context, client *http.Client, panelURLs []string) []panelValidationResult {
	order := CharacterPanelOrder()
	out := make([]panelValidationResult, 0, len(panelURLs))
	for i, u := range panelURLs {
		var panel CharacterPanel
		if i < len(order) {
			panel = order[i]
		}
		r := panelValidationResult{Panel: panel, URL: u}
		if strings.TrimSpace(u) == "" {
			r.Issues = append(r.Issues, "empty url")
			r.Fatal = true
			out = append(out, r)
			continue
		}
		img, err := downloadAndDecodeImage(ctx, client, u)
		if err != nil {
			r.Issues = append(r.Issues, fmt.Sprintf("decode: %v", err))
			r.Fatal = true
			out = append(out, r)
			continue
		}
		b := img.Bounds()
		r.Width = b.Dx()
		r.Height = b.Dy()
		if r.Height > 0 {
			r.AspectRatio = float64(r.Width) / float64(r.Height)
		}
		// 宽高比检查：closeup 约 0.7~1.1；全身 0.45~0.7
		expectedW, expectedH := panelImageAspect(panel)
		if expectedW > 0 && expectedH > 0 {
			expected := float64(expectedW) / float64(expectedH)
			tolerance := 0.25 // ±25% 容差，不同模型会小幅偏移
			if r.AspectRatio < expected*(1-tolerance) || r.AspectRatio > expected*(1+tolerance) {
				r.Issues = append(r.Issues, fmt.Sprintf("aspect %.2f out of expected %.2f±%.0f%%",
					r.AspectRatio, expected, tolerance*100))
			}
		}
		// 亮度直方图：抽样计算平均灰度
		r.Brightness = averageLuma(img)
		if r.Brightness < 15 {
			r.Issues = append(r.Issues, fmt.Sprintf("too dark (luma %.1f)", r.Brightness))
			r.Fatal = true
		} else if r.Brightness > 240 {
			r.Issues = append(r.Issues, fmt.Sprintf("too bright (luma %.1f)", r.Brightness))
			r.Fatal = true
		}
		out = append(out, r)
	}
	return out
}

// averageLuma —— 抽样计算图像 ITU-R BT.601 亮度平均值。
// 对 1024+ 像素宽高的图片按 16x16 网格采样，避免全图遍历。
func averageLuma(img image.Image) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	stepX := w / 64
	stepY := h / 64
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}
	var sum float64
	var count int
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bl, _ := img.At(x, y).RGBA()
			// RGBA() 返回 0..65535，归一化到 0..255
			luma := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			sum += luma
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// collectFatalPanels —— 从验证结果中提取需要重绘的栏索引（相对于 CharacterPanelOrder）。
func collectFatalPanels(results []panelValidationResult) []int {
	order := CharacterPanelOrder()
	idxOf := map[CharacterPanel]int{}
	for i, p := range order {
		idxOf[p] = i
	}
	out := make([]int, 0)
	for _, r := range results {
		if r.Fatal {
			if idx, ok := idxOf[r.Panel]; ok {
				out = append(out, idx)
			}
		}
	}
	return out
}
