package service

import (
	"strings"
	"unicode/utf8"
)

// StoryboardPromptParts represents structured parts of a storyboard prompt
type StoryboardPromptParts struct {
	Subject          string
	OpeningState     string
	Environment      string
	CharacterAnchors []string
	PropAnchors      []string
	SceneAnchors     []string
	EraConstraint    string
	RegionConstraint string
	CameraGrammar    string
	StyleAnchor       string
	ContinuityNote   string
	Negatives        string
}

func extractAssetVisualAnchors(assets []assetReference) (characters, props, scenes []string) {
	for _, asset := range assets {
		anchor := buildAssetAnchor(asset)
		switch strings.ToLower(strings.TrimSpace(asset.Type)) {
		case "character", "char":
			characters = append(characters, anchor)
		case "prop", "item":
			props = append(props, anchor)
		case "scene", "location":
			scenes = append(scenes, anchor)
		}
	}
	return
}

func buildAssetAnchor(asset assetReference) string {
	name := strings.TrimSpace(asset.Name)
	if name == "" {
		return ""
	}
	desc := strings.TrimSpace(asset.Description)
	if desc == "" {
		return name
	}
	maxDescLen := 80
	if utf8.RuneCountInString(desc) > maxDescLen {
		desc = string([]rune(desc)[:maxDescLen]) + "..."
	}
	return name + "(" + desc + ")"
}

func composeStoryboardPrompt(parts StoryboardPromptParts) string {
	var segments []string

	if parts.Subject != "" {
		segments = append(segments, parts.Subject)
	}

	if parts.OpeningState != "" {
		segments = append(segments, "opening: "+parts.OpeningState)
	}

	if parts.Environment != "" {
		segments = append(segments, "environment: "+parts.Environment)
	}

	if len(parts.CharacterAnchors) > 0 {
		segments = append(segments, "characters: "+strings.Join(parts.CharacterAnchors, "; "))
	}
	if len(parts.PropAnchors) > 0 {
		segments = append(segments, "props: "+strings.Join(parts.PropAnchors, "; "))
	}
	if len(parts.SceneAnchors) > 0 {
		segments = append(segments, "scenes: "+strings.Join(parts.SceneAnchors, "; "))
	}

	if parts.EraConstraint != "" {
		segments = append(segments, parts.EraConstraint)
	}
	if parts.RegionConstraint != "" {
		segments = append(segments, parts.RegionConstraint)
	}

	if parts.CameraGrammar != "" {
		segments = append(segments, "camera: "+parts.CameraGrammar)
	}

	if parts.StyleAnchor != "" {
		segments = append(segments, parts.StyleAnchor)
	}

	if parts.ContinuityNote != "" {
		segments = append(segments, "continuity: "+parts.ContinuityNote)
	}

	if parts.Negatives != "" {
		segments = append(segments, "negatives: "+parts.Negatives)
	}

	return strings.Join(segments, ", ")
}

func buildStructuredAssetNote(assets []assetReference) string {
	if len(assets) == 0 {
		return ""
	}
	characters, props, scenes := extractAssetVisualAnchors(assets)
	var parts []string
	if len(characters) > 0 {
		parts = append(parts, "characters: "+strings.Join(characters, "; "))
	}
	if len(props) > 0 {
		parts = append(parts, "props: "+strings.Join(props, "; "))
	}
	if len(scenes) > 0 {
		parts = append(parts, "scenes: "+strings.Join(scenes, "; "))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}
