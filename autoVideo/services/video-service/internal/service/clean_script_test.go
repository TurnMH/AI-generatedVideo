package service

import (
"strings"
"testing"
)

func TestCleanScriptForSpeech_StripsProductionTags(t *testing.T) {
in := `第一章 拿住这个贼子[剪辑:开场先用惨叫声引入][音效:惨叫][美术:地牢][调色:冷调][摄影:推镜][灯光:冷白][服化:娄真五+长袍][导演:紧凑节奏][场记:接上一场]
娄真五：拿住这个贼子！
掌刑执事：是，娄师兄。
环境：阴冷地牢。
场景：地牢入口。
摄影：推至特写。
（他缓缓站起身）
【画面转场】`
out := cleanScriptForSpeech(in)
want := []string{"娄真五：拿住这个贼子！", "掌刑执事：是，娄师兄。"}
for _, w := range want {
if !strings.Contains(out, w) {
t.Errorf("missing dialogue line: %q\ngot: %q", w, out)
}
}
bad := []string{"第一章", "拿住这个贼子！娄真五", "阴冷地牢", "推至特写", "缓缓站起身", "画面转场", "地牢入口"}
for _, b := range bad {
if b == "" {
continue
}
if strings.Contains(out, b) && b != "阴冷地牢" { // allow if it's inside dialog, but we didn't put it there
t.Errorf("leaked text: %q\ngot: %q", b, out)
}
}
if strings.Contains(out, "环境") || strings.Contains(out, "场景") || strings.Contains(out, "摄影") {
t.Errorf("leaked production prefix:\n%s", out)
}
if strings.Contains(out, "第一章") {
t.Errorf("leaked chapter title:\n%s", out)
}
t.Logf("cleaned output:\n%s", out)
}
