package service

import "strings"

// chatCompletionsURL normalizes an OpenAI-compatible base URL or full endpoint
// into a /chat/completions URL. model_db api_endpoint values may already include
// the completions path after migration 013+.
func chatCompletionsURL(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}
