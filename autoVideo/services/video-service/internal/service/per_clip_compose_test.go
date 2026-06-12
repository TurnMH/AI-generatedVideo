package service

import "testing"

func TestShouldAttachExternalDubbing(t *testing.T) {
	dialogues := []string{"旁白：第一句", "", "  "}
	empty := []string{"", "  "}

	if !shouldAttachExternalDubbing(false, false, false, empty) {
		t.Fatal("non-native models should always allow external dubbing")
	}
	if !shouldAttachExternalDubbing(true, true, false, empty) {
		t.Fatal("explicit attach_dubbing should win")
	}
	if shouldAttachExternalDubbing(true, false, true, dialogues) {
		t.Fatal("generate_audio=true should skip external dubbing on native models")
	}
	if !shouldAttachExternalDubbing(true, false, false, dialogues) {
		t.Fatal("dialogues without generate_audio should attach dubbing on native models")
	}
	if shouldAttachExternalDubbing(true, false, false, empty) {
		t.Fatal("native model without dialogues or generate_audio should not attach")
	}
}
