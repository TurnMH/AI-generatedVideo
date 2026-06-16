package service

import "testing"

func TestSanitizeImagePromptMisreadTermsReplacesButterflyLighting(t *testing.T) {
	raw := "面部柔光照明, 蝴蝶光, butterfly lighting, 极致细节"
	got := sanitizeImagePromptMisreadTerms(raw)
	for _, banned := range []string{"蝴蝶光", "butterfly lighting", "butterfly"} {
		if containsIgnoreCase(got, banned) {
			t.Fatalf("sanitized prompt still contains %q: %q", banned, got)
		}
	}
	if !containsIgnoreCase(got, "面中对称柔光棚拍布光") {
		t.Fatalf("expected paramount-style lighting replacement, got %q", got)
	}
}

func TestSanitizeImagePromptMisreadTermsKeepsBowKnot(t *testing.T) {
	raw := "发饰为红色蝴蝶结，蝴蝶光"
	got := sanitizeImagePromptMisreadTerms(raw)
	if !containsIgnoreCase(got, "蝴蝶结") {
		t.Fatalf("should preserve 蝴蝶结, got %q", got)
	}
}

func containsIgnoreCase(text, sub string) bool {
	return len(sub) == 0 || len(text) >= len(sub) && (func() bool {
		lowerText := toLowerASCII(text)
		lowerSub := toLowerASCII(sub)
		for i := 0; i+len(lowerSub) <= len(lowerText); i++ {
			if lowerText[i:i+len(lowerSub)] == lowerSub {
				return true
			}
		}
		return false
	})()
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
