package scriptsplit

import "sort"

// SplitReviewAction is an LLM-suggested fix for episode boundaries.
type SplitReviewAction struct {
	Type         string `json:"type"`
	EpisodeIndex int    `json:"episode_index"`
}

// SplitReviewResult is the LLM response for project-level split review.
type SplitReviewResult struct {
	Passed  bool                `json:"passed"`
	Issues  []SplitReviewIssue  `json:"issues"`
	Actions []SplitReviewAction `json:"actions"`
}

// SplitReviewIssue describes one structural split problem.
type SplitReviewIssue struct {
	Type         string `json:"type"`
	EpisodeIndex int    `json:"episode_index"`
	Severity     string `json:"severity"`
	Detail       string `json:"detail"`
}

// ApplySplitReviewActions applies merge/drop actions in ascending index order.
func ApplySplitReviewActions(episodes []DraftEpisode, actions []SplitReviewAction) []DraftEpisode {
	if len(episodes) == 0 || len(actions) == 0 {
		return episodes
	}

	sort.Slice(actions, func(i, j int) bool {
		if actions[i].EpisodeIndex == actions[j].EpisodeIndex {
			return actions[i].Type < actions[j].Type
		}
		return actions[i].EpisodeIndex < actions[j].EpisodeIndex
	})

	out := append([]DraftEpisode(nil), episodes...)
	offset := 0
	for _, action := range actions {
		idx := action.EpisodeIndex - offset
		if idx < 0 || idx >= len(out) {
			continue
		}
		switch action.Type {
		case "merge_into_next":
			if idx+1 >= len(out) {
				continue
			}
			out[idx] = mergeDraftEpisodes(out[idx], out[idx+1])
			out = append(out[:idx+1], out[idx+2:]...)
			offset++
		case "drop":
			out = append(out[:idx], out[idx+1:]...)
			offset++
		}
	}
	return compactEmptyDraftEpisodes(out)
}
