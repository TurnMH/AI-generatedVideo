package textdecode

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func Decode(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return strings.TrimPrefix(string(raw), "\uFEFF"), nil
	}
	if utf8.Valid(raw) {
		return strings.TrimPrefix(string(raw), "\uFEFF"), nil
	}

	candidates := []encoding.Encoding{
		textunicode.UTF16(textunicode.LittleEndian, textunicode.ExpectBOM),
		textunicode.UTF16(textunicode.BigEndian, textunicode.ExpectBOM),
		simplifiedchinese.GB18030,
		simplifiedchinese.GBK,
		traditionalchinese.Big5,
	}

	var (
		bestText  string
		bestScore = -1.0
		bestErr   error
	)

	for _, enc := range candidates {
		decoded, _, err := transform.String(enc.NewDecoder(), string(raw))
		if err != nil {
			bestErr = err
			continue
		}
		score := score(decoded)
		if score > bestScore {
			bestScore = score
			bestText = decoded
		}
	}

	if bestText == "" {
		if bestErr == nil {
			bestErr = errors.New("unsupported or unknown text encoding")
		}
		return "", bestErr
	}

	return strings.TrimPrefix(bestText, "\uFEFF"), nil
}

func score(text string) float64 {
	if text == "" {
		return 0
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}

	var readable int
	var replacement int
	var badControl int

	for _, r := range runes {
		switch {
		case r == utf8.RuneError:
			replacement++
		case r == '\n' || r == '\r' || r == '\t':
			readable++
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r):
			readable++
		case unicode.Is(unicode.Han, r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			readable++
		case unicode.IsControl(r):
			badControl++
		default:
			readable++
		}
	}

	total := float64(len(runes))
	return (float64(readable) / total) - (float64(replacement) * 1.5 / total) - (float64(badControl) / total)
}
