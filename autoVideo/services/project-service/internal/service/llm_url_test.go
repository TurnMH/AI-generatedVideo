package service

import "testing"

func TestChatCompletionsURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		base string
		want string
	}{
		{base: "https://cld.ppapi.vip/v1", want: "https://cld.ppapi.vip/v1/chat/completions"},
		{base: "https://cld.ppapi.vip/v1/chat/completions", want: "https://cld.ppapi.vip/v1/chat/completions"},
		{base: "https://cld.ppapi.vip/v1/chat/completions/", want: "https://cld.ppapi.vip/v1/chat/completions"},
		{base: "", want: ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.base, func(t *testing.T) {
			t.Parallel()
			if got := chatCompletionsURL(tc.base); got != tc.want {
				t.Fatalf("chatCompletionsURL(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}
}
