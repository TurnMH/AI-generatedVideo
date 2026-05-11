package service

import (
	"strings"
	"testing"

	"github.com/autovideo/video-service/internal/model"
)

func TestMotionPromptIncludesRenderConfigHints(t *testing.T) {
	prompt := motionPrompt("cinematic", "realistic-drama", "", model.RenderConfig{
		"frame_size":   "portrait-9-16",
		"subject_size": "close-up",
		"clarity":      "ultra",
	})

	for _, want := range []string{
		"cinematic slow pan",
		"grounded realistic drama style",
		"stable subject identity",
		"portrait 9:16 composition",
		"close-up framing",
		"ultra clear",
		"avoid flicker",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
}

func TestMotionPromptWithoutRenderConfigKeepsBasePrompt(t *testing.T) {
	prompt := motionPrompt("gentle", "anime", "", nil)
	if strings.Contains(prompt, "portrait 9:16 composition") {
		t.Fatalf("unexpected render config prompt in %q", prompt)
	}
	if !strings.Contains(prompt, "gentle motion") {
		t.Fatalf("expected gentle prompt, got %q", prompt)
	}
}

func TestDescribeVideoNegativePromptForLiveActionAvoidsAnime(t *testing.T) {
	got := describeVideoNegativePrompt("live-action-film")
	for _, want := range []string{"anime", "cartoon", "watermark"} {
		if !strings.Contains(got, want) {
			t.Fatalf("negative prompt %q does not contain %q", got, want)
		}
	}
}

func TestBuildVideoScenePrompt(t *testing.T) {
	desc := `真人电影风格,画面描述 (Screen description): 50年前，长沙镖子岭。一个荒凉的土丘上，四个穿着破旧衣服的土夫子正蹲着。天空阴沉。构图 (Composition): 三角构图。景别 (Shot Scale): 全景 (Full Shot)，展现人物与环境的关系。机位 (Camera Position): 轴线右侧机位。角度 (Angle): 平视 (Eye-Level)。镜头类型 (Lens Type): 广角镜头 (Wide-Angle Lens)，强调环境的荒凉感。光线 (Lighting): 自然光，阴天下的漫射光，色调偏冷。【角色状态追踪】老烟头-动作状态：静止，盯着洛阳铲；老烟头-情绪状态：凝重；大胡子-动作状态：静止；大胡子-情绪状态：紧张；独眼二伢子-动作状态：静止；独眼二伢子-情绪状态：不耐烦；三伢子-动作状态：静止，双手撑地；三伢子-情绪状态：好奇。`

	prompt := buildVideoScenePrompt(desc)

	for _, want := range []string{
		"Full Shot",
		"Eye-Level",
		"Wide-Angle Lens",
		"lighting:",
		"character actions:",
		"emotional tone:",
		"solemn",
		"tense",
		"scene:",
		"50年前",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("scene prompt %q does not contain %q", prompt, want)
		}
	}

	// Section-header labels should NOT appear in the output.
	for _, bad := range []string{"Screen description", "Shot Scale", "Lens Type", "Composition"} {
		if strings.Contains(prompt, bad) {
			t.Fatalf("scene prompt %q should not contain label %q", prompt, bad)
		}
	}
}

func TestResolveTransitionPlanUsesSceneSemantics(t *testing.T) {
	transitions, durations := resolveTransitionPlan(model.RenderConfig{}, 3,
		[]string{"room", "room", "street"},
		[]string{"角色冲向门口", "镜头继续跟随角色冲刺", "夜色街头建立环境"},
		[]string{"向左跟拍", "向左推进", "固定镜头"},
		[]string{"tense", "tense", "calm"},
	)

	if len(transitions) != 2 || len(durations) != 2 {
		t.Fatalf("unexpected transition plan sizes: %v %v", transitions, durations)
	}
	if transitions[0] != "wipeleft" {
		t.Fatalf("first cut = %q, want wipeleft", transitions[0])
	}
	if transitions[1] != "fade" {
		t.Fatalf("second cut = %q, want fade", transitions[1])
	}
	if durations[0] != 0.22 {
		t.Fatalf("first duration = %v, want 0.22", durations[0])
	}
	if durations[1] < 0.32 {
		t.Fatalf("second duration = %v, want >= 0.32", durations[1])
	}
}

func TestResolveTransitionPlanHonorsExplicitOverride(t *testing.T) {
	transitions, durations := resolveTransitionPlan(model.RenderConfig{
		"transition":          "circleclose",
		"transition_duration": 0.7,
	}, 3, []string{"a", "b", "c"}, nil, nil, nil)

	for i, want := range []string{"circleclose", "circleclose"} {
		if transitions[i] != want {
			t.Fatalf("transition[%d] = %q, want %q", i, transitions[i], want)
		}
		if durations[i] != 0.7 {
			t.Fatalf("duration[%d] = %v, want 0.7", i, durations[i])
		}
	}
}

func TestMotionPromptUserPromptIncludesSceneMetadata(t *testing.T) {
	svc := &MotionPromptService{}
	prompt := svc.buildUserPrompt(
		[]string{"角色冲向门口", "镜头贴着人物继续前冲"},
		"主角：红色风衣，短发。",
		[]string{"scene-a", "scene-a"},
		[]string{"向左跟拍", "特写推进"},
		[]string{"tense", "tense"},
		false,
	)

	for _, want := range []string{
		"[scene_group=scene-a]",
		"[same_scene_as_prev=true]",
		"[camera=特写推进]",
		"[mood=tense]",
		"Metadata rule: same_scene_as_prev=true",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
}
