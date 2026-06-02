package service

import "testing"

func TestPostProcessAdScenes_MergesShortDialogueWithoutStructuralShift(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播站在桌前讲解产品卖点，桌面摆着样品",
			Location:    "直播桌前",
			Characters:  []string{"主播"},
			Dialogue:    "这款真的很稳，续航也够。",
			Duration:    5,
		},
		{
			Description: "[中景] 主播继续补一句短收束，动作和场景都没变",
			Location:    "直播桌前",
			Characters:  []string{"主播"},
			Dialogue:    "现在买",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 1 {
		t.Fatalf("expected 1 merged scene, got %d", len(got))
	}
	if got[0].Duration != 5 {
		t.Fatalf("expected merged scene duration normalized to 5, got %d", got[0].Duration)
	}
}

func TestPostProcessAdScenes_KeepsStructuralShiftBoundary(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播在办公室桌前介绍产品",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "这个卖点先给你讲明白。",
			Duration:    5,
		},
		{
			Description: "[远景] 转场到仓库，客户进入新场景并接话",
			Location:    "仓库",
			Characters:  []string{"客户"},
			Dialogue:    "好。",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 2 {
		t.Fatalf("expected structural shift boundary preserved as 2 scenes, got %d", len(got))
	}
	if got[0].Location != "办公室" || got[1].Location != "仓库" {
		t.Fatalf("expected office -> warehouse boundary preserved, got %+v", got)
	}
}

func TestPostProcessAdScenes_MergesShortLeadIntoFollowingSameStructureScene(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播在办公室桌前介绍产品",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "这个卖点先给你讲明白。",
			Duration:    5,
		},
		{
			Description: "[远景] 转场到仓库，客户先接一句短话",
			Location:    "仓库",
			Characters:  []string{"客户"},
			Dialogue:    "好。",
			Duration:    5,
		},
		{
			Description: "[中景] 客户继续在仓库里说明新的使用场景",
			Location:    "仓库",
			Characters:  []string{"客户"},
			Dialogue:    "搬货的时候它也不占地方。",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 2 {
		t.Fatalf("expected short warehouse lead merged into following same-structure scene, got %d", len(got))
	}
	if got[1].Location != "仓库" {
		t.Fatalf("expected merged warehouse scene kept, got %+v", got[1])
	}
}

func TestPostProcessAdScenes_MergesEmptyDialogueIntoPrevious(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播讲第一句",
			Location:    "门店",
			Characters:  []string{"主播"},
			Dialogue:    "现在给你看核心功能。",
			Duration:    5,
		},
		{
			Description: "[近景] 空镜补一个产品特写",
			Location:    "门店",
			Characters:  []string{"主播"},
			Dialogue:    "",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 1 {
		t.Fatalf("expected empty-dialogue scene merged, got %d scenes", len(got))
	}
}

func TestPostProcessAdScenes_MergesShortLeadIntoFollowingScene(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播开场自我介绍，仍在同一办公室",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "大家好，我是李恩泽。",
			Duration:    5,
		},
		{
			Description: "[中景] 主播继续在办公室展开完整卖点说明",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "近年来，市场变化日新月异，人工智能正以前所未有的速度重塑各行各业。",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 1 {
		t.Fatalf("expected short lead merged into following scene, got %d scenes", len(got))
	}
}

func TestPostProcessAdScenes_NormalizesAllDurationsToSelectedClipDuration(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 主播完整介绍第一段卖点",
			Location:    "展台",
			Characters:  []string{"主播"},
			Dialogue:    "第一段口播内容足够完整，需要单独成镜。",
			Duration:    4,
		},
		{
			Description: "[中景] 主播继续讲第二段卖点",
			Location:    "展台",
			Characters:  []string{"主播"},
			Dialogue:    "第二段口播同样完整，也需要单独成镜。",
			Duration:    8,
		},
	}
	got := svc.postProcessAdScenes(scenes, 6)
	if len(got) != 2 {
		t.Fatalf("expected 2 scenes kept, got %d", len(got))
	}
	for i, scene := range got {
		if scene.Duration != 6 {
			t.Fatalf("expected scene %d duration normalized to 6, got %d", i, scene.Duration)
		}
	}
}

func TestPostProcessAdScenes_RebalancesShortMiddleIntoLongFollowingScene(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "[中景] 第一段完整介绍",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "近年来，市场变化日新月异，人工智能正以前所未有的速度重塑各行各业。",
			Duration:    5,
		},
		{
			Description: "[中景] 同场景下一个过短承接句",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "从AI到半导体。",
			Duration:    5,
		},
		{
			Description: "[中景] 同场景继续展开大段解释",
			Location:    "办公室",
			Characters:  []string{"主播"},
			Dialogue:    "再到全球资本流向，新的机遇层出不穷，但真正洞察其背后逻辑的人却屈指可数。",
			Duration:    5,
		},
	}
	got := svc.postProcessAdScenes(scenes, 5)
	if len(got) != 2 {
		t.Fatalf("expected short middle scene merged for rebalance, got %d scenes", len(got))
	}
}
