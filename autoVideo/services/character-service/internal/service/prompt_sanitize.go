package service

import "strings"

// misreadLightingReplacements —— 部分模型会把「蝴蝶光 / butterfly lighting」字面画成蝴蝶。
// 统一替换为不含 butterfly 字样的棚拍布光描述。
var misreadLightingReplacements = [][2]string{
	{"蝴蝶光照明", "面中对称柔光棚拍布光"},
	{"蝴蝶光", "面中对称柔光棚拍布光"},
	{"butterfly lighting setup", "paramount studio lighting setup"},
	{"butterfly lighting", "paramount studio lighting"},
	{"butterfly light", "soft front studio key light"},
	{"Butterfly Lighting", "paramount studio lighting"},
	{"Butterfly lighting", "paramount studio lighting"},
}

// characterInsectNegativeTerms —— 角色设定图常见误生成：蓝色蝴蝶/昆虫装饰。
func characterInsectNegativeTerms() []string {
	return []string{
		"butterfly", "blue butterfly", "butterflies", "moth", "insect", "insects",
		"bug", "dragonfly", "animal companion", "pet insect",
		"蝴蝶", "蓝色蝴蝶", "蓝蝴蝶", "昆虫", "飞蛾", "蝶", "动物",
	}
}

// sanitizeImagePromptMisreadTerms rewrites lighting phrases that models often literalize,
// and strips orphan 「蝴蝶」 tokens left after replacement.
func sanitizeImagePromptMisreadTerms(prompt string) string {
	text := strings.TrimSpace(prompt)
	if text == "" {
		return text
	}
	for _, pair := range misreadLightingReplacements {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}
	// 若仍残留独立「蝴蝶」且语境像布光/修饰语，直接去掉，避免触发误生成。
	if strings.Contains(text, "蝴蝶") && !strings.Contains(text, "蝴蝶结") {
		text = strings.ReplaceAll(text, "，蝴蝶", "")
		text = strings.ReplaceAll(text, ", butterfly", "")
		text = strings.ReplaceAll(text, " butterfly", "")
		text = strings.ReplaceAll(text, "蝴蝶", "")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}
