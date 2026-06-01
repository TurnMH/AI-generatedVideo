package service

import (
	"strings"
	"unicode/utf8"
)

// StoryboardPromptParts represents structured parts of a storyboard prompt
type StoryboardPromptParts struct {
	Subject            string
	OpeningState       string
	Environment        string
	CharacterAnchors   []string
	PropAnchors        []string
	SceneAnchors       []string
	EraConstraint      string
	RegionConstraint   string
	CameraGrammar      string
	StyleAnchor        string
	ContinuityNote     string
	PoseConstraint     string
	ActionConstraint   string
	SpatialConstraint  string
	WardrobeConstraint string
	Negatives          string
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

	if parts.PoseConstraint != "" {
		segments = append(segments, "pose_lock: "+parts.PoseConstraint)
	}
	if parts.ActionConstraint != "" {
		segments = append(segments, "action_chain: "+parts.ActionConstraint)
	}
	if parts.SpatialConstraint != "" {
		segments = append(segments, "spatial_lock: "+parts.SpatialConstraint)
	}
	if parts.WardrobeConstraint != "" {
		segments = append(segments, "wardrobe_lock: "+parts.WardrobeConstraint)
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

func deriveStoryboardConstraintHints(req StoryboardGenerateRequest, originalCharacters []string, charAppearances, charAssetPrompts map[string]string) StoryboardPromptParts {
	texts := []string{strings.TrimSpace(req.PromptUsed), strings.TrimSpace(req.SceneDescription)}
	poseHint := collectSceneHintFromTexts(texts, []string{
		"站", "坐", "俯身", "抬手", "抬眼", "低头", "回头", "侧身", "弯腰", "扶", "端", "抱", "跪", "站姿", "坐姿", "姿态",
		"standing", "seated", "leaning", "kneeling", "crouching", "turning", "facing", "shoulders", "posture", "stance", "gesture", "pose", "arms", "hands",
	})
	actionHint := collectSceneHintFromTexts(texts, []string{
		"走", "转身", "推门", "拿起", "放下", "递给", "指向", "展示", "讲解", "靠近", "离开", "停下", "迈步", "起身", "坐下", "开口",
		"walk", "step", "turn", "reach", "hold", "present", "point", "open", "close", "move", "pause", "approach", "leave", "lift", "place", "speaking",
	})
	spatialParts := []string{
		collectSceneHintFromTexts(texts, []string{"左侧", "右侧", "居中", "中央", "左前方", "右前方", "左后方", "右后方", "面朝", "背对", "对视", "靠近", "left", "right", "center", "foreground", "background", "midground", "facing", "screen left", "screen right"}),
		collectSceneHintFromTexts(texts, []string{"门", "窗", "桌", "椅", "沙发", "床", "楼梯", "柜台", "车", "路口", "走廊", "墙边", "窗边", "门口", "door", "window", "table", "desk", "sofa", "bed", "stairs", "counter", "hallway"}),
		collectSceneHintFromTexts(texts, []string{"走近", "后退", "转身", "绕过", "停下", "起身", "坐下", "推门", "回头", "移步", "迈步", "靠近", "离开", "穿过", "steps closer", "backs away", "circles", "stops by", "turns from", "crosses"}),
	}
	spatialHint := joinNonEmptyUnique(spatialParts, " | ")

	wardrobeSource := collectSceneHintFromTexts(texts, []string{
		"服饰", "服装", "外套", "西装", "衬衫", "裙", "长裙", "短裙", "裤", "牛仔", "制服", "高跟", "运动鞋", "项链", "耳环", "发夹", "发型", "妆容", "配饰",
		"outfit", "wardrobe", "jacket", "blazer", "shirt", "dress", "skirt", "pants", "trousers", "uniform", "heels", "sneakers", "necklace", "earrings", "hairstyle", "makeup", "accessories",
	})
	wardrobeHint := strings.TrimSpace(wardrobeSource)
	if wardrobeHint == "" {
		var names []string
		for i, name := range req.Characters {
			lookupKey := strings.ToLower(strings.TrimSpace(name))
			if i < len(originalCharacters) && strings.TrimSpace(originalCharacters[i]) != "" {
				lookupKey = strings.ToLower(strings.TrimSpace(originalCharacters[i]))
			}
			desc := strings.TrimSpace(charAssetPrompts[lookupKey])
			if desc == "" {
				desc = strings.TrimSpace(charAppearances[lookupKey])
			}
			if desc != "" {
				names = append(names, strings.TrimSpace(name))
			}
		}
		if len(names) > 0 {
			wardrobeHint = "Preserve the same garment layers, dominant colors, accessories, hair styling, and makeup from the canonical character lock for " + strings.Join(names, ", ")
		}
	}

	return StoryboardPromptParts{
		PoseConstraint:     poseHint,
		ActionConstraint:   actionHint,
		SpatialConstraint:  spatialHint,
		WardrobeConstraint: wardrobeHint,
	}
}

func collectSceneHintFromTexts(texts []string, keywords []string) string {
	var picks []string
	for _, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		if hit := collectSceneHintByKeywords(trimmed, keywords); hit != "" {
			picks = append(picks, hit)
		}
	}
	return joinNonEmptyUnique(picks, " | ")
}

func joinNonEmptyUnique(parts []string, sep string) string {
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return strings.Join(out, sep)
}
