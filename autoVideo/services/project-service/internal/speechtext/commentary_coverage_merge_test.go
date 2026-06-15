package speechtext

import "testing"

func TestMergeAdjacentSpeechUnits(t *testing.T) {
	units := []string{
		"王总，你说个数吧",
		"刘师傅，这次来，是专程道歉的",
		"我抽了口烟，没说话。那碗汤浇得好。",
	}
	got := MergeAdjacentSpeechUnits(units, 20)
	if len(got) >= len(units) {
		t.Fatalf("expected fewer merged units, got %v", got)
	}
}

func TestPackSpeechUnitsToMaxRunes_MinChunkSize(t *testing.T) {
	units := []string{
		"王总，你说个数吧",
		"刘师傅，这次来，是专程道歉的",
		"这样，我给您三万一个月，另外入股百分之五，逢年过节另有红包",
	}
	got := PackSpeechUnitsToMaxRunes(units, 38)
	for i, chunk := range got {
		if len([]rune(chunk)) < 18 && i < len(got)-1 {
			t.Fatalf("chunk %d too short: %q", i, chunk)
		}
	}
}
