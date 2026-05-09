package handler

import (
	"testing"

	"github.com/autovideo/project-service/pkg/textdecode"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDecodeScriptTextUTF8(t *testing.T) {
	const source = "盗墓笔记\n第一章 七星鲁王宫"

	decoded, err := textdecode.Decode([]byte(source))
	if err != nil {
		t.Fatalf("decode utf8: %v", err)
	}
	if decoded != source {
		t.Fatalf("decoded utf8 mismatch: got %q want %q", decoded, source)
	}
}

func TestDecodeScriptTextGBK(t *testing.T) {
	const source = "盗墓笔记\n第一章 七星鲁王宫"

	raw, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(source))
	if err != nil {
		t.Fatalf("encode gbk: %v", err)
	}

	decoded, err := textdecode.Decode(raw)
	if err != nil {
		t.Fatalf("decode gbk: %v", err)
	}
	if decoded != source {
		t.Fatalf("decoded gbk mismatch: got %q want %q", decoded, source)
	}
}
