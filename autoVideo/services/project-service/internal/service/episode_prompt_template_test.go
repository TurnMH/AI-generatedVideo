package service

import (
	"reflect"
	"testing"
)

func TestStoryboardTemplateLookupKeys(t *testing.T) {
	t.Run("anime2d falls back to legacy style key", func(t *testing.T) {
		got := storyboardTemplateLookupKeys("storyboard_anime2d")
		want := []string{"storyboard_anime2d", "animation_v43"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("storyboardTemplateLookupKeys() = %v, want %v", got, want)
		}
	})

	t.Run("other styles stay exact", func(t *testing.T) {
		got := storyboardTemplateLookupKeys("storyboard_cinematic")
		want := []string{"storyboard_cinematic"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("storyboardTemplateLookupKeys() = %v, want %v", got, want)
		}
	})
}

func TestApplyPromptTemplate(t *testing.T) {
	template := "scene={scene}; chars={characters}; action={action}; mood={mood}"
	got := applyPromptTemplate(template, "rainy alley", "Li Ming, Chen Yu", "draws sword", "tense")
	want := "scene=rainy alley; chars=Li Ming, Chen Yu; action=draws sword; mood=tense"
	if got != want {
		t.Fatalf("applyPromptTemplate() = %q, want %q", got, want)
	}
}