package speechtext

import (
	"regexp"
	"strings"
)

var reInlineAnnotation = regexp.MustCompile(`[（(【\[]([^:：\[\]（）【】]{1,12})[：:]([^\]）】)]*)[）)\]】]`)

var scriptStripTags = map[string]bool{
	"摄影": true, "镜头": true, "画面": true, "景别": true, "视角": true, "构图": true, "机位": true, "运镜": true,
	"音效": true, "声效": true, "配乐": true, "背景音": true, "音乐": true, "音响": true, "拟音": true,
	"转场": true, "过渡": true, "淡入": true, "淡出": true, "剪辑": true, "衔接": true,
	"场景": true, "地点": true, "背景": true, "布景": true, "道具": true, "美术": true, "置景": true,
	"时间": true, "时段": true, "时代": true, "朝代": true, "年代": true, "环境": true, "天气": true,
	"动作": true, "表情": true, "肢体": true, "走位": true, "调度": true, "情绪": true, "氛围": true, "基调": true,
	"灯光": true, "光线": true, "打光": true, "调色": true, "色彩": true, "色调": true,
	"服化": true, "服装": true, "妆造": true, "造型": true, "发型": true, "化妆": true, "服饰": true, "人物": true, "演员": true,
	"导演": true, "场记": true, "制片": true, "录音": true, "剧本": true, "动画": true, "特效": true, "后期": true, "监制": true, "灯光师": true,
	"旁注": true, "说明": true, "备注": true, "注意": true, "提示": true, "字幕特效": true, "字幕样式": true, "标注": true, "注释": true,
	"节奏": true, "时长": true, "秒数": true, "镜头时长": true,
}

var scriptSpeechTags = map[string]bool{
	"字幕": true, "对白": true, "台词": true, "独白": true, "旁白": true, "内心独白": true, "画外音": true, "解说": true,
}

var linePrefixStripPattern = regexp.MustCompile(`^(?:摄影|镜头|画面|景别|视角|构图|机位|运镜|音效|声效|配乐|背景音|音乐|音响|拟音|转场|过渡|淡入|淡出|剪辑|衔接|场景|地点|背景|布景|道具|美术|置景|时间|时段|时代|朝代|年代|环境|天气|动作|表情|肢体|走位|调度|情绪|氛围|基调|灯光|光线|打光|调色|色彩|色调|服化|服装|妆造|造型|发型|化妆|服饰|演员|导演|场记|制片|录音|剧本|动画|特效|后期|监制|旁注|说明|备注|注意|提示|标注|注释|节奏|时长|秒数|字幕特效|字幕样式)\s*[：:]`)

var chapterTitlePattern = regexp.MustCompile(`^第[一二三四五六七八九十百千零〇两0-9\d]+[章集场幕回部卷节篇]`)

var sceneSluglinePattern = regexp.MustCompile(`^(?:【\s*)?(?:内景|外景|内外景|INT\.?|EXT\.?)(?:\s*[·．.、，,/\|｜\-—–]\s*|\s+).+(?:\s*】)?$`)

var speakerLinePattern = regexp.MustCompile(`^(?:[【\[(（]\s*)?([^:：\]）)】]{1,24})(?:\s*[】\])）])?\s*[:：]\s*(.+)$`)

var screenplayActionLine = regexp.MustCompile(`^[\p{Han}A-Za-z0-9·\s]{1,16}[（(][^)）]{0,30}[）)]\s*[\p{Han}A-Za-z]`)

var speakerCueOnlyPattern = regexp.MustCompile(`^(?:旁白|主持人|主播|解说|老师|嘉宾|男声|女声|人物|角色|画外音|OS|VO)(?:\s*(?:[（(][^)）]{0,30}[）)]|\([^)]{0,30}\)))?\s*$`)

var speakerWithEmotionLine = regexp.MustCompile(`^(.+?)[（(][^)）]{0,24}[）)]\s*[:：]\s*(.+)$`)

var nameOnlyLinePattern = regexp.MustCompile(`^[\p{Han}A-Za-z·]{1,8}[。！?？]?$`)

var sceneSettingLinePattern = regexp.MustCompile(`^(?:[\p{Han}A-Za-z0-9·]{2,30}[，,]\s*)?(?:清晨|早晨|早上|上午|中午|午后|傍晚|黄昏|夜里|夜晚|夜间|深夜|凌晨|日间|日出|日落)[，,]`)

var locationLeadLinePattern = regexp.MustCompile(`^[\p{Han}]{2,}(?:楼|堂|馆|店|院|房|室|厨|厅|街|巷|路|园|场|殿|宫|城|村|镇|山|河|湖|海|门|间|内|外|里|中)[，,]`)

var actionOnlyLinePattern = regexp.MustCompile(`^[\p{Han}]{1,8}(?:缓缓|慢慢|轻轻|忽然|猛然|转身|抬头|低头|走|跑|拿|放|推|拉|看|望|站|坐|蹲|靠|握|举|切|揉|炒|煮|递|接|挥|指|叹|笑|哭|愣|震|顿|沉默|专注).+[。！]?$`)

var timeTransitionLinePattern = regexp.MustCompile(`^(?:\d+年[前后]?|\d+个?月[前后]?|三天后|翌日|次日|同时|此时|那一刻|三个月前|一年前|数日后|片刻后)[。！]?$`)

// SanitizeForSpeech strips production annotations and stage directions so only speakable dialogue remains.
func SanitizeForSpeech(text string) string {
	text = reInlineAnnotation.ReplaceAllStringFunc(text, func(match string) string {
		parts := reInlineAnnotation.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		tag := strings.TrimSpace(parts[1])
		content := strings.TrimSpace(parts[2])
		if scriptSpeechTags[tag] {
			return content
		}
		if scriptStripTags[tag] {
			return ""
		}
		return ""
	})

	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if (strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")")) ||
			(strings.HasPrefix(line, "（") && strings.HasSuffix(line, "）")) ||
			(strings.HasPrefix(line, "【") && strings.HasSuffix(line, "】")) ||
			(strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
			continue
		}
		if sceneSluglinePattern.MatchString(line) || strings.HasPrefix(line, "△") {
			continue
		}
		if speakerCueOnlyPattern.MatchString(line) {
			continue
		}
		if screenplayActionLine.MatchString(line) && !speakerLinePattern.MatchString(line) {
			continue
		}
		if nameOnlyLinePattern.MatchString(line) ||
			sceneSettingLinePattern.MatchString(line) ||
			locationLeadLinePattern.MatchString(line) ||
			actionOnlyLinePattern.MatchString(line) ||
			timeTransitionLinePattern.MatchString(line) {
			continue
		}
		if m := speakerWithEmotionLine.FindStringSubmatch(line); len(m) == 3 {
			line = strings.TrimSpace(m[1]) + "：" + strings.TrimSpace(m[2])
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "***") {
			continue
		}
		if chapterTitlePattern.MatchString(line) || linePrefixStripPattern.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
