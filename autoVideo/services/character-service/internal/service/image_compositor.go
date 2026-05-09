package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // png 解码
	"io"
	"net/http"
	"time"

	"golang.org/x/image/draw"
)

const (
	// 标准化每栏高度；宽度按原图比例缩放。closeup 拼接前会做居中裁切 / padding 以贴合整体高度。
	compositeTargetHeight = 1280
	// 栏间白色分隔宽度。
	compositeGutterPx = 16
	// jpeg 输出质量（0-100）。
	compositeJPEGQuality = 92
	// 单张图最大下载体积，防御恶意响应。
	compositeMaxBytes = 40 << 20 // 40MB
)

// composeHorizontalPanels 下载 urls 指向的 N 张图，统一高度后横向拼接为一张 JPEG。
// 顺序直接使用 urls 的顺序（调用方需按 characterPanelOrder 传入）。
// 任一张下载/解码失败即返回错误，调用方决定是否降级或重试。
func composeHorizontalPanels(ctx context.Context, client *http.Client, urls []string) ([]byte, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("composeHorizontalPanels: empty urls")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	images := make([]image.Image, len(urls))
	for i, u := range urls {
		img, err := downloadAndDecodeImage(ctx, client, u)
		if err != nil {
			return nil, fmt.Errorf("panel %d (%s): %w", i, u, err)
		}
		images[i] = img
	}

	// 归一化高度：所有图片缩放到 compositeTargetHeight。
	resized := make([]image.Image, len(images))
	totalWidth := 0
	for i, img := range images {
		b := img.Bounds()
		srcW, srcH := b.Dx(), b.Dy()
		if srcH <= 0 || srcW <= 0 {
			return nil, fmt.Errorf("panel %d has invalid size %dx%d", i, srcW, srcH)
		}
		// 等比例缩放到目标高度。
		scale := float64(compositeTargetHeight) / float64(srcH)
		dstW := int(float64(srcW) * scale)
		if dstW <= 0 {
			dstW = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, dstW, compositeTargetHeight))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		resized[i] = dst
		totalWidth += dstW
	}

	// 加入栏间白条。
	totalWidth += compositeGutterPx * (len(resized) - 1)

	canvas := image.NewRGBA(image.Rect(0, 0, totalWidth, compositeTargetHeight))
	// 纯白底。
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	xOffset := 0
	for i, panel := range resized {
		pb := panel.Bounds()
		dstRect := image.Rect(xOffset, 0, xOffset+pb.Dx(), compositeTargetHeight)
		draw.Draw(canvas, dstRect, panel, pb.Min, draw.Over)
		xOffset += pb.Dx()
		if i < len(resized)-1 {
			xOffset += compositeGutterPx
		}
	}

	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, canvas, &jpeg.Options{Quality: compositeJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode composite jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// downloadAndDecodeImage 下载 url 并解码为 image.Image（自动识别 jpeg/png）。
func downloadAndDecodeImage(ctx context.Context, client *http.Client, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, compositeMaxBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(buf)) > compositeMaxBytes {
		return nil, fmt.Errorf("image too large (> %d bytes)", compositeMaxBytes)
	}
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}
