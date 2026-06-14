package service

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	inlineActionTagPattern = regexp.MustCompile(`\[动作[:：]([^\]]+)\]`)
)

// EnrichSceneDescriptionForVideo keeps storyboard narrative text as the primary
// video prompt source and appends speakable dialogue when it is not already present.
func EnrichSceneDescriptionForVideo(sceneDesc, dialogue string) string {
	sceneDesc = strings.TrimSpace(sceneDesc)
	dialogue = strings.TrimSpace(dialogue)
	if sceneDesc == "" && dialogue == "" {
		return ""
	}
	if sceneDesc == "" {
		return "剧情节拍：" + dialogue
	}
	if dialogue == "" {
		return sceneDesc
	}
	if strings.Contains(sceneDesc, dialogue) {
		return sceneDesc
	}
	return sceneDesc + "。本镜旁白/对白：" + dialogue
}

// MergeStoryAndMotionPrompt combines narrative scene text with motion refinement.
// Story content always comes first so video models follow the script beat.
func MergeStoryAndMotionPrompt(story, motion string) string {
	story = strings.TrimSpace(story)
	motion = strings.TrimSpace(motion)
	if story == "" {
		return motion
	}
	if motion == "" {
		return story
	}
	if strings.Contains(story, motion) || motionOverlapRatio(story, motion) >= 0.55 {
		return story
	}
	if hasMeaningfulChinese(story) {
		return story + "。" + motion
	}
	return story + ". " + motion
}

func preparePerClipStoryDescriptions(perClipDescs, _ []string) []string {
	out := make([]string, len(perClipDescs))
	for i := range perClipDescs {
		out[i] = compactVideoSceneDescription(perClipDescs[i])
	}
	return out
}

func compactVideoSceneDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ""
	}
	if actions := extractInlineStoryTags(desc); len(actions) > 0 {
		return strings.Join(actions, "。") + "。"
	}
	return pruneVideoAppearanceCatalog(desc)
}

var videoAppearanceNoisePattern = regexp.MustCompile(`(?:身穿|身着|穿着|发型|黑发|花白|脸型|圆润|商人气息|环境光线|背景简洁|神情|面露|身形对比|气氛紧张|近景突出|远景)`)

func pruneVideoAppearanceCatalog(desc string) string {
	desc = strings.ReplaceAll(desc, "；", "，")
	clauses := strings.Split(desc, "，")
	kept := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		if videoAppearanceNoisePattern.MatchString(clause) && !videoClauseHasAction(clause) {
			continue
		}
		kept = append(kept, clause)
	}
	if len(kept) == 0 {
		return strings.TrimSpace(desc)
	}
	out := strings.Join(kept, "，")
	if !strings.HasSuffix(out, "。") {
		out += "。"
	}
	return out
}

func videoClauseHasAction(clause string) bool {
	for _, verb := range []string{"揉", "站", "走", "拿", "放", "推", "拉", "看", "转", "抬", "低", "开", "关", "切", "递", "接", "坐", "蹲", "跑", "握", "拍", "敲", "揉面", "开门", "探头"} {
		if strings.Contains(clause, verb) {
			return true
		}
	}
	return false
}

func applyMotionPromptRefinement(
	svc *MotionPromptService,
	ctx context.Context,
	storyDescs []string,
	modelFamily, motionMode, stylePreset, charDescriptions string,
	sceneGroupKeys, cameraHints, moodHints, spatialAnchors, subjectPositions, transitionNotes []string,
) []string {
	if svc == nil || len(storyDescs) == 0 {
		return storyDescs
	}
	refined := svc.RefineBatch(
		ctx,
		storyDescs,
		modelFamily,
		motionMode,
		stylePreset,
		charDescriptions,
		sceneGroupKeys,
		cameraHints,
		moodHints,
		spatialAnchors,
		subjectPositions,
		transitionNotes,
	)
	if len(refined) == 0 {
		return storyDescs
	}
	out := make([]string, len(storyDescs))
	for i := range storyDescs {
		motion := ""
		if i < len(refined) {
			motion = refined[i]
		}
		out[i] = MergeStoryAndMotionPrompt(storyDescs[i], motion)
	}
	return out
}

func extractInlineStoryTags(desc string) []string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var parts []string
	for _, re := range []*regexp.Regexp{inlineActionTagPattern} {
		for _, m := range re.FindAllStringSubmatch(desc, -1) {
			if len(m) < 2 {
				continue
			}
			text := strings.TrimSpace(m[1])
			if text == "" {
				continue
			}
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			parts = append(parts, text)
		}
	}
	return parts
}

func motionOverlapRatio(a, b string) float64 {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return 0
	}
	shorter, longer := a, b
	if utf8.RuneCountInString(a) > utf8.RuneCountInString(b) {
		shorter, longer = b, a
	}
	runes := []rune(shorter)
	if len(runes) < 16 {
		return 0
	}
	window := 48
	if len(runes) < window {
		window = len(runes)
	}
	sample := string(runes[:window])
	longerLen := utf8.RuneCountInString(longer)
	if longerLen < 1 {
		longerLen = 1
	}
	if strings.Contains(longer, sample) {
		return float64(window) / float64(longerLen)
	}
	return 0
}
