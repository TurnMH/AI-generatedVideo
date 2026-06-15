package service

import (
	"strings"
	"testing"
)

func TestSanitizeUserSceneDescription_StripsBoilerplate(t *testing.T) {
	in := "刘师傅在灶台前揉面，面粉飞散。镜头衔接：承接上一镜头，保持同一场景空间方位、布景陈设与光线色温。氛围：平和宁静，自然柔光，舒缓节奏。"
	out := sanitizeUserSceneDescription(in)
	if !strings.Contains(out, "刘师傅在灶台前揉面") {
		t.Fatalf("expected core description kept, got %q", out)
	}
	if strings.Contains(out, "镜头衔接") || strings.Contains(out, "氛围：") {
		t.Fatalf("expected boilerplate removed, got %q", out)
	}
}

func TestInferSceneDurationFromDialogue_ShortNarration(t *testing.T) {
	got := inferSceneDurationFromDialogue("他当时确实有点尴尬呢", 8, "normal")
	if got >= 8 {
		t.Fatalf("expected shorter clip for short narration, got %d", got)
	}
	if got < 2 {
		t.Fatalf("duration too short: %d", got)
	}
}

func TestInferSceneDurationFromDialogue_ShortCharacterLine(t *testing.T) {
	got := inferSceneDurationFromDialogue("王大发：还是您来扛，您看——", 8, "normal")
	if got >= 6 {
		t.Fatalf("expected short clip for brief character line, got %d", got)
	}
	if got < 2 {
		t.Fatalf("duration too short: %d", got)
	}
}

func TestInferSceneDurationFromDialogue_LongNarration(t *testing.T) {
	long := strings.Repeat("这是一个需要更长时间才能念完的旁白句子。", 4)
	got := inferSceneDurationFromDialogue(long, 5, "with_pauses")
	if got <= 5 {
		t.Fatalf("expected longer clip for long narration with pauses, got %d", got)
	}
}

func TestEnrichSceneDescription_NoSpatialBridge(t *testing.T) {
	prev := &llmScene{Location: "后厨", Description: "灶台边"}
	scene := llmScene{
		Description: "刘师傅抬头看向门口。",
		Location:    "后厨",
		Characters:  []string{"刘师傅"},
		Mood:        "calm",
	}
	out := enrichSceneDescription(scene, prev, nil, "民国初年")
	if strings.Contains(out, "镜头衔接") || strings.Contains(out, "空间方位") {
		t.Fatalf("unexpected spatial boilerplate: %q", out)
	}
	if !strings.Contains(out, "刘师傅抬头看向门口") {
		t.Fatalf("expected original description preserved, got %q", out)
	}
}
