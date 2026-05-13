package handler

import "testing"

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