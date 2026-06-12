package service

import (
	"fmt"
	"math"
	"strings"
)

const audioSyncAtempoMaxDrift = 0.18

func audioVideoDriftRatio(videoDur, audioDur float64) float64 {
	if videoDur <= 0 || audioDur <= 0 {
		return 0
	}
	return math.Abs(videoDur-audioDur) / math.Max(videoDur, audioDur)
}

// buildAtempoChain builds an ffmpeg atempo filter chain for speedRatio (>0).
// speedRatio > 1 speeds audio up; < 1 slows it down.
func buildAtempoChain(speedRatio float64) string {
	if speedRatio <= 0 {
		return "atempo=1"
	}
	if math.Abs(speedRatio-1) < 0.005 {
		return "atempo=1"
	}

	var speeds []float64
	remaining := speedRatio
	for remaining > 2.0+1e-6 {
		speeds = append(speeds, 2.0)
		remaining /= 2.0
	}
	for remaining < 0.5-1e-6 {
		speeds = append(speeds, 0.5)
		remaining /= 0.5
	}
	speeds = append(speeds, remaining)

	parts := make([]string, 0, len(speeds))
	for _, speed := range speeds {
		parts = append(parts, fmt.Sprintf("atempo=%.4f", speed))
	}
	return strings.Join(parts, ",")
}
