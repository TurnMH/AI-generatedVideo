package productionmode

import (
	"encoding/json"
	"strings"

	"github.com/autovideo/project-service/internal/model"
)

// Mode identifies which production pipeline a project should follow.
type Mode string

const (
	ModeAd              Mode = "ad"
	ModeCommentaryComic Mode = "commentary_comic"
	ModeScriptDrama     Mode = "script_drama"
	ModeComics          Mode = "comics"
)

// Profile describes pipeline behavior for a resolved mode.
type Profile struct {
	Mode Mode
}

func (p Profile) IsAd() bool              { return p.Mode == ModeAd }
func (p Profile) IsCommentaryComic() bool { return p.Mode == ModeCommentaryComic }
func (p Profile) IsScriptDrama() bool     { return p.Mode == ModeScriptDrama }
func (p Profile) IsComics() bool          { return p.Mode == ModeComics }

// ShouldOptimizeScriptBeforeSplit returns true only for ad projects that explicitly
// enable auto_split_after_optimization in storyboard_config.
func (p Profile) ShouldOptimizeScriptBeforeSplit(autoSplitAfterOptimization bool) bool {
	return p.IsAd() && autoSplitAfterOptimization
}

// ShouldPostProcessMergeScenes returns true when scene lists should be merged for
// ad-style voiceover duration constraints.
func (p Profile) ShouldPostProcessMergeScenes() bool {
	return p.IsAd()
}

// ShouldSkipScriptPrep returns true when pre-split script LLM prep should be skipped.
func (p Profile) ShouldSkipScriptPrep() bool {
	return p.IsComics()
}

// UseAdEpisodeEstimate returns true when episode count estimation should apply
// ad-specific chars-per-clip heuristics.
func (p Profile) UseAdEpisodeEstimate() bool {
	return p.IsAd()
}

// UseAdSimpleSplit returns true when fallback episode splitting should preserve
// ad semantic boundaries (卖点/CTA/转场).
func (p Profile) UseAdSimpleSplit() bool {
	return p.IsAd()
}

// Resolve determines the production mode from project metadata.
// Priority: ad-workbench tag > comics type > storyboard_config.production_mode > commentary tag > script drama default.
func Resolve(project *model.Project) Mode {
	if project == nil {
		return ModeScriptDrama
	}
	if hasStyleTag(project, "ad-workbench") {
		return ModeAd
	}
	if strings.EqualFold(strings.TrimSpace(project.ProjectType), "comics") {
		return ModeComics
	}
	if configured := configuredProductionMode(project); configured != "" {
		return configured
	}
	if hasCommentaryTag(project) {
		return ModeCommentaryComic
	}
	return ModeScriptDrama
}

func configuredProductionMode(project *model.Project) Mode {
	if project == nil || len(project.StoryboardConfig) == 0 {
		return ""
	}
	var cfg struct {
		ProductionMode string `json:"production_mode"`
	}
	if err := json.Unmarshal(project.StoryboardConfig, &cfg); err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ProductionMode)) {
	case string(ModeCommentaryComic), "commentary", "commentary-comic":
		return ModeCommentaryComic
	case string(ModeScriptDrama), "script", "script-drama", "drama":
		return ModeScriptDrama
	default:
		return ""
	}
}

// ResolveProfile returns the behavior profile for a project.
func ResolveProfile(project *model.Project) Profile {
	return Profile{Mode: Resolve(project)}
}

// IsAd is a convenience helper for callers that only need ad detection.
func IsAd(project *model.Project) bool {
	return Resolve(project) == ModeAd
}

func hasStyleTag(project *model.Project, tag string) bool {
	want := strings.ToLower(strings.TrimSpace(tag))
	for _, raw := range project.StyleTags {
		if strings.ToLower(strings.TrimSpace(raw)) == want {
			return true
		}
	}
	return false
}

func hasCommentaryTag(project *model.Project) bool {
	for _, raw := range project.StyleTags {
		lower := strings.ToLower(strings.TrimSpace(raw))
		if lower == "解说漫" || lower == "commentary_comic" || lower == "explainer-comic" {
			return true
		}
	}
	return false
}
