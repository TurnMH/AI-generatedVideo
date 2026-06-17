package service

import (
	"regexp"
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
			lookupName := strings.TrimSpace(name)
			if i < len(originalCharacters) && strings.TrimSpace(originalCharacters[i]) != "" {
				lookupName = strings.TrimSpace(originalCharacters[i])
			}
			desc := lookupCharacterAssetText(lookupName, charAssetPrompts)
			if desc == "" {
				desc = lookupCharacterAssetText(lookupName, charAppearances)
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

// buildMultiCharacterBlockingCue extracts per-character action clauses from scene text so
// multi-ref image models do not swap poses (e.g. who kneels vs who stands inside).
func buildMultiCharacterBlockingCue(characters []string, sceneDesc, spatialAnchor string) string {
	if len(characters) < 2 {
		return ""
	}
	text := strings.TrimSpace(sceneDesc)
	if anchor := strings.TrimSpace(spatialAnchor); anchor != "" {
		text = joinNonEmptyUnique([]string{text, anchor}, " | ")
	}
	text = enrichMultiCharacterBlockingText(text, characters)
	if text == "" {
		return ""
	}
	segments := regexp.MustCompile(`[，。；！？\n|]`).Split(text, -1)
	var blocks []string
	for _, name := range characters {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || !strings.Contains(seg, name) {
			continue
		}
		blocks = append(blocks, name+": "+seg)
		break
	}
	}
	if len(blocks) < 2 {
		narrator := pickCommentaryPOVCharacter(characters)
		if narrator != "" {
			for _, seg := range segments {
				seg = strings.TrimSpace(seg)
				if seg == "" || !strings.Contains(seg, narrator) {
					continue
				}
				found := false
				for _, b := range blocks {
					if strings.HasPrefix(b, narrator+":") {
						found = true
						break
					}
				}
				if !found {
					blocks = append(blocks, narrator+": "+seg)
				}
				break
			}
		}
	}
	if len(blocks) < 2 {
		return ""
	}
	return strings.Join(blocks, "; ")
}

func enrichStoryboardImagePromptWithConstraints(
	prompt string,
	source StoryboardGenerateRequest,
	originalCharacters []string,
	charAppearances, charAssetPrompts map[string]string,
) string {
	hints := deriveStoryboardConstraintHints(source, originalCharacters, charAppearances, charAssetPrompts)
	spatial := joinNonEmptyUnique([]string{
		hints.SpatialConstraint,
		strings.TrimSpace(source.SpatialAnchor),
		strings.TrimSpace(source.TransitionNote),
		parseStructuredPromptField(source.PromptUsed, "spatial_lock"),
	}, " | ")
	pose := joinNonEmptyUnique([]string{
		hints.PoseConstraint,
		strings.TrimSpace(source.SubjectPositions),
		parseStructuredPromptField(source.PromptUsed, "pose_lock"),
	}, " | ")

	var extras []string
	if cue := storyboardCameraCue(source.CameraMovement); cue != "" {
		extras = append(extras, cue)
	}
	if cue := entranceSplitCompositionCue(source.LocationZone, source.SceneDescription); cue != "" {
		extras = append(extras, cue)
	}
	if pose != "" {
		extras = append(extras, "Pose and body language (must match exactly): "+truncateForImagePrompt(pose, 280)+".")
	}
	if spatial != "" {
		extras = append(extras, "Spatial blocking (do not swap characters or locations): "+truncateForImagePrompt(spatial, 280)+".")
	}
	if blocking := buildMultiCharacterBlockingCue(originalCharacters, source.SceneDescription, source.SpatialAnchor); blocking != "" {
		extras = append(extras, "Per-character blocking: "+truncateForImagePrompt(blocking, 320)+".")
	}
	if len(originalCharacters) >= 2 {
		extras = append(extras, "Two or more distinct characters: use reference images for face/outfit identity only; follow the scene-specific poses above, not the reference sheet standing pose; never swap who kneels, stands, or speaks.")
	}
	if hints.ActionConstraint != "" {
		extras = append(extras, "Action beat: "+truncateForImagePrompt(hints.ActionConstraint, 200)+".")
	}
	if len(extras) == 0 {
		return prompt
	}
	return strings.TrimSpace(prompt + " " + strings.Join(extras, " "))
}

func parseStructuredPromptField(promptUsed, field string) string {
	promptUsed = strings.TrimSpace(promptUsed)
	field = strings.TrimSpace(field)
	if promptUsed == "" || field == "" {
		return ""
	}
	prefix := field + ":"
	var parts []string
	for _, seg := range strings.Split(promptUsed, ",") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, prefix) {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(seg, prefix)))
		}
	}
	return strings.Join(parts, " | ")
}

func entranceSplitCompositionCue(locationZone, sceneDesc string) string {
	zone := strings.ToLower(strings.TrimSpace(locationZone))
	text := strings.TrimSpace(sceneDesc)
	if text == "" {
		return ""
	}
	if zone != "entrance" && zone != LocationViewEntrance {
		return ""
	}
	hasExterior := strings.Contains(text, "门外") || strings.Contains(text, "街边") || strings.Contains(text, "门口") || strings.Contains(text, "external")
	hasInterior := strings.Contains(text, "铺内") || strings.Contains(text, "店内") || strings.Contains(text, "室内") || strings.Contains(text, "内景")
	if !hasExterior || !hasInterior {
		return ""
	}
	return "Composition: doorway split-frame — exterior foreground shows the kneeling/approaching figure at the shop entrance; interior background shows the second figure inside under warm shop light through the open doorway. Keep both identities distinct and do not merge them into one pose."
}

func formatCharacterStatesForBlocking(states []llmCharacterState) string {
	if len(states) == 0 {
		return ""
	}
	parts := make([]string, 0, len(states))
	for _, cs := range states {
		name := strings.TrimSpace(cs.Name)
		if name == "" {
			continue
		}
		seg := name
		if action := strings.TrimSpace(cs.Action); action != "" {
			seg += "：" + action
		}
		if emotion := strings.TrimSpace(cs.Emotion); emotion != "" {
			if strings.Contains(seg, "：") {
				seg += "，" + emotion
			} else {
				seg += "：" + emotion
			}
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "；")
}

func characterNamesFromAssets(assets []assetReference) []string {
	if len(assets) == 0 {
		return nil
	}
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		if !strings.EqualFold(strings.TrimSpace(asset.Type), "character") {
			continue
		}
		if name := strings.TrimSpace(asset.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func appendMultiCharacterPoseNegativeTokens(baseNeg string) string {
	extras := []string{
		"swapped characters", "role reversal", "wrong person kneeling", "wrong person standing",
		"merged identities", "duplicate same person", "character pose swap", "fused bodies",
	}
	if strings.TrimSpace(baseNeg) == "" {
		return strings.Join(extras, ", ")
	}
	return baseNeg + ", " + strings.Join(extras, ", ")
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
