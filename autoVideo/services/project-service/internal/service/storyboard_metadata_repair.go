package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/model"
)

type StoryboardMetadataRepairResult struct {
	Repaired           int `json:"repaired"`
	CharactersFilled   int `json:"characters_filled"`
	SpatialAnchorFixed int `json:"spatial_anchor_fixed"`
	SubjectPositionsFixed int `json:"subject_positions_fixed"`
	PromptsCleared     int `json:"prompts_cleared"`
}

func isPoorSceneHintSegment(seg string) bool {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return true
	}
	if strings.Contains(seg, "【导语】") || strings.HasPrefix(seg, "【") {
		return true
	}
	if utf8.RuneCountInString(seg) > 96 {
		return true
	}
	if strings.Count(seg, "。") >= 2 || strings.Count(seg, "，") >= 4 {
		return true
	}
	// Dialogue / narration dumps with broken quotes, not visual anchors.
	if strings.Contains(seg, "：\"") || strings.Contains(seg, ": \"") {
		return true
	}
	if strings.ContainsAny(seg, "「」") && !strings.ContainsAny(seg, "站") && !strings.ContainsAny(seg, "跪") && !strings.ContainsAny(seg, "坐") {
		return true
	}
	if strings.HasPrefix(seg, "他") && strings.ContainsAny(seg, `"`) {
		return true
	}
	// First-person narration fragments are not stable pose anchors in multi-char shots.
	if strings.Contains(seg, "我") && (strings.ContainsAny(seg, `"`) || strings.ContainsAny(seg, "「」")) {
		return true
	}
	if strings.HasPrefix(seg, "我") && utf8.RuneCountInString(seg) <= 24 {
		return true
	}
	return false
}

func isCorruptedStoryboardPromptUsed(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "redacted_thinking") || strings.Contains(lower, "<think") {
		return true
	}
	if strings.Count(s, "Single 2D anime") > 1 || strings.Count(s, "Keep character identity consistent with:") > 1 {
		return true
	}
	if utf8.RuneCountInString(s) > 6000 {
		return true
	}
	return false
}

func collectProjectCharacterCatalog(storyboards []model.Storyboard, extra []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(extra)+8)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	for _, name := range extra {
		add(name)
	}
	for _, sb := range storyboards {
		for _, name := range sb.Characters {
			add(name)
		}
	}
	return out
}

// RepairEpisodeMetadata backfills missing characters, rebuilds spatial/subject hints from
// scene descriptions, and clears corrupted prompt_used values for an episode.
func (s *StoryboardService) RepairEpisodeMetadata(ctx context.Context, projectID uint64, episodeID *uint64, characterCatalog []string) (*StoryboardMetadataRepairResult, error) {
	_ = ctx
	statuses := []string{"pending", "generating", "paused", "completed", "failed"}
	projectStoryboards, err := s.repo.FindByProjectAndStatuses(projectID, nil, statuses, 100000)
	if err != nil {
		return nil, err
	}
	targets, err := s.repo.FindByProjectAndStatuses(projectID, episodeID, statuses, 100000)
	if err != nil {
		return nil, err
	}
	catalog := collectProjectCharacterCatalog(projectStoryboards, characterCatalog)
	result := &StoryboardMetadataRepairResult{}
	for i := range targets {
		sb := &targets[i]
		if sb.IsVoided {
			continue
		}
		changed := false
		if len(sb.Characters) == 0 {
			inferred := inferStoryboardCharacters(nil, sb.SceneDescription, sb.Dialogue, catalog)
			if len(inferred) > 0 {
				sb.Characters = inferred
				changed = true
				result.CharactersFilled++
			}
		}
		if isCommentaryDialoguePollutedDescription(sb.SceneDescription) {
			repaired := repairCommentarySceneDescription(sb.SceneDescription, sb.Dialogue, llmScene{
				Description: sb.SceneDescription,
				Dialogue:    sb.Dialogue,
				Location:    sb.Location,
				Characters:  sb.Characters,
			}, pickCommentaryPOVCharacter(sb.Characters))
			if repaired != "" && repaired != sb.SceneDescription {
				sb.SceneDescription = repaired
				changed = true
			}
		}
		if rebuilt := extractSpatialAnchorHint(sb.SceneDescription); rebuilt != "" && (sb.SpatialAnchor == "" || isPoorSceneHintSegment(sb.SpatialAnchor)) {
			if sb.SpatialAnchor != rebuilt {
				sb.SpatialAnchor = rebuilt
				changed = true
				result.SpatialAnchorFixed++
			}
		} else if sb.SpatialAnchor != "" && isPoorSceneHintSegment(sb.SpatialAnchor) {
			sb.SpatialAnchor = ""
			changed = true
			result.SpatialAnchorFixed++
		}
		if rebuilt := extractSubjectPositionHint(sb.SceneDescription); rebuilt != "" && (sb.SubjectPositions == "" || isPoorSceneHintSegment(sb.SubjectPositions)) {
			if sb.SubjectPositions != rebuilt {
				sb.SubjectPositions = rebuilt
				changed = true
				result.SubjectPositionsFixed++
			}
		} else if sb.SubjectPositions != "" && isPoorSceneHintSegment(sb.SubjectPositions) {
			sb.SubjectPositions = ""
			changed = true
			result.SubjectPositionsFixed++
		}
		if normSpatial, normSubject := normalizeStoryboardPoseHints(sb.SpatialAnchor, sb.SubjectPositions, sb.Characters); normSpatial != sb.SpatialAnchor || normSubject != sb.SubjectPositions {
			sb.SpatialAnchor = normSpatial
			sb.SubjectPositions = normSubject
			changed = true
			result.SubjectPositionsFixed++
		}
		if !sb.PromptLocked && isCorruptedStoryboardPromptUsed(sb.PromptUsed) {
			sb.PromptUsed = ""
			changed = true
			result.PromptsCleared++
		}
		if changed {
			sb.UpdatedAt = time.Now()
			if err := s.repo.Update(sb); err != nil {
				return result, err
			}
			result.Repaired++
		}
	}
	return result, nil
}
