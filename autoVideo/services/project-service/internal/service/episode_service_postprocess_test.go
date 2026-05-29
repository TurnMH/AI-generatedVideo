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
