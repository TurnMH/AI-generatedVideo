package service

import (
	"testing"

	"github.com/autovideo/video-service/internal/model"
)

func TestBuildAtempoChain(t *testing.T) {
	if got := buildAtempoChain(1); got != "atempo=1" {
		t.Fatalf("got %q", got)
	}
	if got := buildAtempoChain(1.2); got != "atempo=1.2000" {
		t.Fatalf("got %q", got)
	}
	if got := buildAtempoChain(2.4); got != "atempo=2.0000,atempo=1.2000" {
		t.Fatalf("got %q", got)
	}
}

func TestAudioVideoDriftRatio(t *testing.T) {
	if audioVideoDriftRatio(5, 5.5) > audioSyncAtempoMaxDrift {
		t.Fatal("expected drift within atempo threshold")
	}
	if audioVideoDriftRatio(5, 7) <= audioSyncAtempoMaxDrift {
		t.Fatal("expected large drift outside atempo threshold")
	}
}

func TestInferSemanticTransition_SameSceneGroupUsesDissolve(t *testing.T) {
	transition, dur := inferSemanticTransition(
		0,
		[]string{"kitchen-a", "kitchen-a"},
		nil, nil, nil,
		0.5,
	)
	if transition != "dissolve" {
		t.Fatalf("got transition %q want dissolve", transition)
	}
	if dur < 0.55 {
		t.Fatalf("expected longer dissolve duration, got %v", dur)
	}
}

func TestResolveTransitionPlan_SameSceneGroupLongerExplicitDissolve(t *testing.T) {
	cfg := modelRenderConfig(map[string]any{
		"transition":          "dissolve",
		"transition_duration": 0.5,
	})
	transitions, durations := resolveTransitionPlan(
		cfg,
		3,
		[]string{"scene-a", "scene-a", "scene-b"},
		nil, nil, nil,
	)
	if len(transitions) != 2 || transitions[0] != "dissolve" {
		t.Fatalf("unexpected transitions: %v", transitions)
	}
	if durations[0] < 0.65 {
		t.Fatalf("same-scene dissolve should be longer, got %v", durations[0])
	}
	if durations[1] != 0.5 {
		t.Fatalf("cross-scene should keep fallback duration, got %v", durations[1])
	}
}

func modelRenderConfig(values map[string]any) model.RenderConfig {
	out := make(model.RenderConfig, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}
