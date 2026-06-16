package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestBuildEpisodeExtractionChunksKeepsEpisodesSeparate(t *testing.T) {
	t.Parallel()

	episodes := []episodeSummary{
		{ID: 1, Number: 1, Title: "第一集", Excerpt: "刘小楼在地牢中醒来。"},
		{ID: 2, Number: 2, Title: "第二集", Excerpt: "韩无望在山林小道追击对手。"},
	}

	chunks := buildEpisodeExtractionChunks(episodes)
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}
	if len(chunks[0].EpisodeIDs) != 1 || chunks[0].EpisodeIDs[0] != 1 {
		t.Fatalf("chunks[0].EpisodeIDs = %v, want [1]", chunks[0].EpisodeIDs)
	}
	if len(chunks[1].EpisodeIDs) != 1 || chunks[1].EpisodeIDs[0] != 2 {
		t.Fatalf("chunks[1].EpisodeIDs = %v, want [2]", chunks[1].EpisodeIDs)
	}
	if chunks[0].Text == chunks[1].Text {
		t.Fatal("episode extraction chunks should not collapse multiple episodes into the same text chunk")
	}
}

func TestUserFacingExtractionError(t *testing.T) {
	t.Parallel()

	if got := userFacingExtractionError(fmt.Errorf("llm call: context deadline exceeded")); !strings.Contains(got, "超时") {
		t.Fatalf("userFacingExtractionError() = %q, want timeout hint", got)
	}
	if got := userFacingExtractionError(errors.New("no assets extracted from episode")); !strings.Contains(got, "未能从剧本中识别") {
		t.Fatalf("userFacingExtractionError() = %q, want empty extraction hint", got)
	}
}

func TestBuildExtractionRoutesIncludesFallback(t *testing.T) {
	t.Parallel()

	svc := &ExtractService{
		llmBaseURL:      "https://primary.example/v1",
		llmAPIKey:       "primary-key",
		llmModel:        "gpt-5.4",
		fallbackBaseURL: "https://fallback.example/v2",
		fallbackAPIKey:  "fallback-key",
		fallbackModel:   "glm-5",
	}
	routes := svc.buildExtractionRoutes(context.Background(), "")
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2", len(routes))
	}
	if routes[0].model != "gpt-5.4" || routes[1].model != "glm-5" {
		t.Fatalf("routes = %+v, want primary then fallback", routes)
	}
}