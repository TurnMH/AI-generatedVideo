package productionmode

import (
	"testing"

	"github.com/autovideo/project-service/internal/model"
	"github.com/lib/pq"
)

func TestResolve_AdWorkbench(t *testing.T) {
	project := &model.Project{StyleTags: pq.StringArray{"ad-workbench"}, ProjectType: "video"}
	if got := Resolve(project); got != ModeAd {
		t.Fatalf("expected %s, got %s", ModeAd, got)
	}
}

func TestResolve_CommentaryComic(t *testing.T) {
	project := &model.Project{StyleTags: pq.StringArray{"解说漫", "漫画"}, ProjectType: "video"}
	if got := Resolve(project); got != ModeCommentaryComic {
		t.Fatalf("expected %s, got %s", ModeCommentaryComic, got)
	}
}

func TestResolve_ComicsType(t *testing.T) {
	project := &model.Project{ProjectType: "comics"}
	if got := Resolve(project); got != ModeComics {
		t.Fatalf("expected %s, got %s", ModeComics, got)
	}
}

func TestResolve_ScriptDramaDefault(t *testing.T) {
	project := &model.Project{ProjectType: "video", StyleTags: pq.StringArray{"仙侠"}}
	if got := Resolve(project); got != ModeScriptDrama {
		t.Fatalf("expected %s, got %s", ModeScriptDrama, got)
	}
}

func TestProfile_ShouldSkipEpisodeScriptOptimization(t *testing.T) {
	commentary := Profile{Mode: ModeCommentaryComic}
	if !commentary.ShouldSkipEpisodeScriptOptimization() {
		t.Fatal("commentary comic should skip episode script optimization")
	}
	if !commentary.ShouldSkipScriptPrep() {
		t.Fatal("commentary comic should skip script prep before split")
	}
	drama := Profile{Mode: ModeScriptDrama}
	if drama.ShouldSkipEpisodeScriptOptimization() {
		t.Fatal("script drama should not skip episode script optimization")
	}
}

func TestProfile_ShouldOptimizeScriptBeforeSplit(t *testing.T) {
	ad := Profile{Mode: ModeAd}
	if !ad.ShouldOptimizeScriptBeforeSplit(true) {
		t.Fatal("ad project with flag should optimize")
	}
	if ad.ShouldOptimizeScriptBeforeSplit(false) {
		t.Fatal("ad project without flag should not optimize")
	}
	script := Profile{Mode: ModeScriptDrama}
	if script.ShouldOptimizeScriptBeforeSplit(true) {
		t.Fatal("script drama should never use ad optimization")
	}
}

func TestBuildAutoSplitMeta_AdUsesHigherChars(t *testing.T) {
	script := string(make([]rune, 1000))
	runtime := RuntimeConfig{Duration: 5, AutoSplitAfterOptimization: true}
	adMeta := BuildAutoSplitMeta(script, runtime, Profile{Mode: ModeAd})
	dramaMeta := BuildAutoSplitMeta(script, runtime, Profile{Mode: ModeScriptDrama})
	if adMeta.TargetCharsPerEpisode <= dramaMeta.TargetCharsPerEpisode {
		t.Fatalf("ad target chars should be larger: ad=%d drama=%d", adMeta.TargetCharsPerEpisode, dramaMeta.TargetCharsPerEpisode)
	}
}

func TestResolveProfile_SkipFlags(t *testing.T) {
	// Test style tags
	projectWithTags := &model.Project{
		StyleTags: pq.StringArray{"direct-split"},
	}
	profile := ResolveProfile(projectWithTags)
	if !profile.SkipScriptOptimization || !profile.SkipPostProcessing {
		t.Fatal("expected direct-split style tag to set SkipScriptOptimization and SkipPostProcessing")
	}

	// Test storyboard_config JSON
	projectWithConfig := &model.Project{
		StoryboardConfig: []byte(`{"direct_split": true}`),
	}
	profile2 := ResolveProfile(projectWithConfig)
	if !profile2.SkipScriptOptimization || !profile2.SkipPostProcessing {
		t.Fatal("expected direct_split config to set SkipScriptOptimization and SkipPostProcessing")
	}

	projectWithIndividualConfig := &model.Project{
		StoryboardConfig: []byte(`{"skip_script_optimization": true, "skip_post_processing": false}`),
	}
	profile3 := ResolveProfile(projectWithIndividualConfig)
	if !profile3.SkipScriptOptimization || profile3.SkipPostProcessing {
		t.Fatal("expected individual config to set SkipScriptOptimization and not SkipPostProcessing")
	}
}
