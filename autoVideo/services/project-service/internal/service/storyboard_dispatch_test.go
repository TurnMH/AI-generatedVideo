package service

import "testing"

func TestStoryboardEligibleDispatchStatusesIncludesFailed(t *testing.T) {
	t.Parallel()

	if len(storyboardEligibleDispatchStatuses) != 2 {
		t.Fatalf("len = %d, want 2", len(storyboardEligibleDispatchStatuses))
	}
	if storyboardEligibleDispatchStatuses[0] != "pending" || storyboardEligibleDispatchStatuses[1] != "failed" {
		t.Fatalf("statuses = %#v, want [pending failed]", storyboardEligibleDispatchStatuses)
	}
}
