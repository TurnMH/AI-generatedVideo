package handler

import (
	"testing"

	"github.com/autovideo/video-service/internal/model"
)

func TestValidateSerialScenePayloadAcceptsReadyGroups(t *testing.T) {
	err := validateSerialScenePayload(
		[]string{"https://example.com/1.png", "", "https://example.com/2.png"},
		[]string{"scene-a", "scene-a", ""},
		true,
	)
	if err != nil {
		t.Fatalf("validateSerialScenePayload() error = %v, want nil", err)
	}
}

func TestValidateSerialScenePayloadRejectsMissingGroupAnchor(t *testing.T) {
	err := validateSerialScenePayload(
		[]string{"", "https://example.com/2.png"},
		[]string{"scene-a", "scene-a"},
		true,
	)
	if err == nil || err.Error() != "serial_scene group \"scene-a\" is missing its first-frame image" {
		t.Fatalf("validateSerialScenePayload() error = %v", err)
	}
}

func TestValidateSerialScenePayloadRejectsEmptyUngroupedClip(t *testing.T) {
	err := validateSerialScenePayload(
		[]string{"https://example.com/1.png", ""},
		[]string{"scene-a", ""},
		true,
	)
	if err == nil || err.Error() != "serial_scene clip 2 must provide a first-frame image when scene_group_key is empty" {
		t.Fatalf("validateSerialScenePayload() error = %v", err)
	}
}

func TestValidateSerialScenePayloadRejectsLengthMismatch(t *testing.T) {
	err := validateSerialScenePayload(
		[]string{"https://example.com/1.png"},
		[]string{},
		true,
	)
	if err == nil || err.Error() != "serial_scene requires scene_group_keys for every clip" {
		t.Fatalf("validateSerialScenePayload() error = %v", err)
	}
}

func TestNormalizeContinuityRenderConfigStoresConstraintFields(t *testing.T) {
	rc := normalizeContinuityRenderConfig(model.RenderConfig{},
		[]string{"办公室窗边", "书房门口"},
		[]string{"人物居右朝左", "人物居中回头"},
		[]string{"承接上一镜头视线", "保持服装动作连续"},
		[][]string{{"speaker"}, {"speaker", "assistant"}},
	)

	if got, ok := rc["config_version"]; !ok || got != videoRenderConfigVersion {
		t.Fatalf("config_version = %v, ok=%v", got, ok)
	}
	if got, ok := rc["spatial_anchors"].([]string); !ok || len(got) != 2 || got[0] != "办公室窗边" {
		t.Fatalf("spatial_anchors = %#v", rc["spatial_anchors"])
	}
	if got, ok := rc["subject_positions"].([]string); !ok || len(got) != 2 || got[1] != "人物居中回头" {
		t.Fatalf("subject_positions = %#v", rc["subject_positions"])
	}
	if got, ok := rc["transition_notes"].([]string); !ok || len(got) != 2 || got[0] != "承接上一镜头视线" {
		t.Fatalf("transition_notes = %#v", rc["transition_notes"])
	}
	if got, ok := rc["scene_characters"].([][]string); !ok || len(got) != 2 || len(got[1]) != 2 || got[1][1] != "assistant" {
		t.Fatalf("scene_characters = %#v", rc["scene_characters"])
	}
}

func TestBuildClipDebugSummariesReadsConstraintFieldsFromRenderConfig(t *testing.T) {
	task := &model.VideoTask{
		RenderConfig: model.RenderConfig{
			"spatial_anchors":   []string{"办公室窗边", "书房门口"},
			"subject_positions": []string{"人物居右朝左", "人物居中回头"},
			"transition_notes":  []string{"承接上一镜头视线", "保持服装动作连续"},
		},
		Clips: []model.VideoClip{
			{ClipOrder: 0},
			{ClipOrder: 1},
		},
	}

	items := buildClipDebugSummaries(task)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if got := items[0]["spatial_anchor"]; got != "办公室窗边" {
		t.Fatalf("clip0 spatial_anchor = %v", got)
	}
	if got := items[1]["subject_positions"]; got != "人物居中回头" {
		t.Fatalf("clip1 subject_positions = %v", got)
	}
	if got := items[1]["transition_note"]; got != "保持服装动作连续" {
		t.Fatalf("clip1 transition_note = %v", got)
	}

	summary := buildTaskDebugSummary(task)
	if got := summary["clip_with_spatial_hints"]; got != 2 {
		t.Fatalf("clip_with_spatial_hints = %v", got)
	}
	if got := summary["clip_with_position_hints"]; got != 2 {
		t.Fatalf("clip_with_position_hints = %v", got)
	}
	if got := summary["clip_with_transition_hints"]; got != 2 {
		t.Fatalf("clip_with_transition_hints = %v", got)
	}
}
