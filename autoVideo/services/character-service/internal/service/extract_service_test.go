package service

import "testing"

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