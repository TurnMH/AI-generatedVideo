package service

import (
	"strings"
)

// AssetPromptParts represents structured parts of an asset image prompt
type AssetPromptParts struct {
	SubjectName string
	SubjectDesc string
	Appearance string
	Wardrobe string
	Accessories string
	EraConstraint string
	RegionConstraint string
	EthnicityConstraint string
	StyleAnchor string
	MotionCue string
	PromptSupplement string
	AnitimalesBanned string
	Environmental string
	Overlays string
	Quality string
}

// composeAssetPrompt builds a structured asset prompt
func composeAssetPrompt(parts AssetPromptParts) string {
	var segments []string
	if parts.SubjectName != "" {
		segments = append(segments, parts.SubjectName)
	}
	if parts.SubjectDesc != "" {
		segments = append(segments, parts.SubjectDesc)
	}
	if parts.Appearance != "" {
		segments = append(segments, parts.Appearance)
	}
	if parts.Wardrobe != "" {
		segments = append(segments, parts.Wardrobe)
	}
	if parts.Accessories != "" {
		segments = append(segments, parts.Accessories)
	}
	if parts.EraConstraint != "" {
		segments = append(segments, parts.EraConstraint)
	}
	if parts.RegionConstraint != "" {
		segments = append(segments, parts.RegionConstraint)
	}
	if parts.EthnicityConstraint != "" {
		segments = append(segments, parts.EthnicityConstraint)
	}
	if parts.StyleAnchor != "" {
		segments = append(segments, parts.StyleAnchor)
	}
	if parts.MotionCue != "" {
		segments = append(segments, parts.MotionCue)
	}
	if parts.PromptSupplement != "" {
		segments = append(segments, parts.PromptSupplement)
	}
	if parts.AnitimalesBanned != "" {
		segments = append(segments, "avoid: "+parts.AnitimalesBanned)
	}
	if parts.Environmental != "" {
		segments = append(segments, "avoid: "+parts.Environmental)
	}
	if parts.Overlays != "" {
		segments = append(segments, "avoid: "+parts.Overlays)
	}
	if parts.Quality != "" {
		segments = append(segments, parts.Quality)
	}
	return strings.Join(segments, ", ")
}

// dedupeParts removes duplicate strings while preserving order
func dedupeParts(parts []string) []string {
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(p))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, p)
	}
	return result
}
