package productionmode

import (
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/stylepreset"
)

// RuntimeConfig carries storyboard runtime fields used for auto-split estimation.
type RuntimeConfig struct {
	Duration                   int
	VideoModel                 string
	StylePreset                string
	AutoSplitAfterOptimization bool
}

// AutoSplitMeta mirrors the progress payload used during episode auto-split.
type AutoSplitMeta struct {
	Enabled               bool   `json:"enabled,omitempty"`
	Duration              int    `json:"duration,omitempty"`
	VideoModel            string `json:"video_model,omitempty"`
	StylePreset           string `json:"style_preset,omitempty"`
	ScriptLength          int    `json:"script_length,omitempty"`
	TargetCharsPerEpisode int    `json:"target_chars_per_episode,omitempty"`
	EstimatedEpisodes     int    `json:"estimated_episodes,omitempty"`
	OriginalScript        string `json:"original_script,omitempty"`
	OptimizedScript       string `json:"optimized_script,omitempty"`
	ConsistencyPremise    string `json:"consistency_premise,omitempty"`
}

// BuildAutoSplitMeta estimates episode count from script length and runtime settings.
func BuildAutoSplitMeta(scriptText string, runtimeCfg RuntimeConfig, profile Profile) AutoSplitMeta {
	scriptLength := utf8.RuneCountInString(strings.TrimSpace(scriptText))
	meta := AutoSplitMeta{
		Enabled:      true,
		Duration:     runtimeCfg.Duration,
		VideoModel:   strings.TrimSpace(runtimeCfg.VideoModel),
		StylePreset:  stylepreset.Canonical(runtimeCfg.StylePreset),
		ScriptLength: scriptLength,
	}
	if meta.Duration <= 0 {
		meta.Duration = 10
	}
	if meta.StylePreset == "" {
		meta.StylePreset = stylepreset.Default
	}

	clipDuration := meta.Duration
	if clipDuration < 3 {
		clipDuration = 3
	}
	if clipDuration > 180 {
		clipDuration = 180
	}

	charsPerSecond := charsPerSecondForPreset(meta.StylePreset)
	modelKey := strings.ToLower(meta.VideoModel)
	switch {
	case strings.Contains(modelKey, "seedance"), strings.Contains(modelKey, "doubao"):
		charsPerSecond -= 1
	case strings.Contains(modelKey, "gaga"):
		charsPerSecond += 1
	}
	if charsPerSecond < 4 {
		charsPerSecond = 4
	}

	baseTargetChars := clipDuration * charsPerSecond
	if baseTargetChars < 80 {
		baseTargetChars = 80
	}
	if profile.UseAdEpisodeEstimate() && runtimeCfg.AutoSplitAfterOptimization {
		adMinChars := clipDuration * 14
		if adMinChars < 140 {
			adMinChars = 140
		}
		if baseTargetChars < adMinChars {
			baseTargetChars = adMinChars
		}
	}
	if baseTargetChars > 4000 {
		baseTargetChars = 4000
	}
	meta.TargetCharsPerEpisode = baseTargetChars

	if scriptLength <= 0 {
		meta.EstimatedEpisodes = 1
		return meta
	}

	meta.EstimatedEpisodes = (scriptLength + meta.TargetCharsPerEpisode - 1) / meta.TargetCharsPerEpisode
	if meta.EstimatedEpisodes < 1 {
		meta.EstimatedEpisodes = 1
	}
	if meta.EstimatedEpisodes > 200 {
		meta.EstimatedEpisodes = 200
	}
	return meta
}

func charsPerSecondForPreset(preset string) int {
	switch preset {
	case stylepreset.LiveActionFilm:
		return 6
	case stylepreset.LiveActionShort:
		return 7
	case stylepreset.Anime3D:
		return 8
	default:
		return 8
	}
}
