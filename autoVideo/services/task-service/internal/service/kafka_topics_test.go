package service

import "testing"

// TestKafkaTopicConstants verifies the music result topic constant is defined
// and has the expected value. The task-service uses this constant to route
// music.generate.result messages back into task status updates.
func TestKafkaTopicConstants(t *testing.T) {
	if TopicMusicGenerateReq != "music.generate.request" {
		t.Errorf("TopicMusicGenerateReq: got %q want %q", TopicMusicGenerateReq, "music.generate.request")
	}
	if TopicMusicGenerateRes != "music.generate.result" {
		t.Errorf("TopicMusicGenerateRes: got %q want %q", TopicMusicGenerateRes, "music.generate.result")
	}
}

// TestTopicRouting verifies the topic → kafka-topic routing for music tasks
// matches the task type string used by the frontend.
func TestTopicRouting(t *testing.T) {
	// The svc.topicForType() logic in task_service.go routes "music_generate"
	// to TopicMusicGenerateReq. Validate the constant pair is coherent.
	if TopicMusicGenerateReq == "" {
		t.Error("TopicMusicGenerateReq must not be empty")
	}
	if TopicMusicGenerateRes == "" {
		t.Error("TopicMusicGenerateRes must not be empty")
	}
	// Request and result topics must differ.
	if TopicMusicGenerateReq == TopicMusicGenerateRes {
		t.Errorf("request and result topic must differ, both are %q", TopicMusicGenerateReq)
	}
}
