package service

// SensitiveEntry maps a problematic word/phrase to a safe visual equivalent that
// preserves the scene's visual intent while passing typical AI image API content filters.
type SensitiveEntry struct {
	Word        string // lower-case word or phrase to match (substring match)
	Replacement string // safe replacement text
	Category    string // "violence" | "sexual" | "cn_violence" | "cn_sexual" | "cn_sensitive"
}

// defaultSensitiveEntries is the built-in list for AI image/video prompt sanitization.
// Replacements keep the cinematic intent intact.
var defaultSensitiveEntries = []SensitiveEntry{
	// ─── English: Violence / Gore ───────────────────────────────────────────────
	{Word: "blood", Replacement: "crimson fluid", Category: "violence"},
	{Word: "gore", Replacement: "dramatic aftermath", Category: "violence"},
	{Word: "mutilat", Replacement: "transformation sequence", Category: "violence"},
	{Word: "decapitat", Replacement: "action cutaway", Category: "violence"},
	{Word: "torture", Replacement: "intense confrontation", Category: "violence"},
	{Word: "corpse", Replacement: "motionless figure", Category: "violence"},
	{Word: "dead body", Replacement: "fallen figure", Category: "violence"},
	{Word: "massacre", Replacement: "aftermath of conflict", Category: "violence"},
	{Word: "slaughter", Replacement: "fierce battle", Category: "violence"},
	{Word: "brutal killing", Replacement: "dramatic confrontation", Category: "violence"},
	{Word: "dismember", Replacement: "dark silhouette", Category: "violence"},
	{Word: "beheading", Replacement: "swift action", Category: "violence"},

	// ─── English: Explicit Sexual Content ───────────────────────────────────────
	{Word: "nude", Replacement: "wearing flowing robes", Category: "sexual"},
	{Word: "naked", Replacement: "lightly clothed", Category: "sexual"},
	{Word: "nsfw", Replacement: "dramatic", Category: "sexual"},
	{Word: "pornograph", Replacement: "intimate scene", Category: "sexual"},
	{Word: "explicit sexual", Replacement: "romantic", Category: "sexual"},
	{Word: "erotic", Replacement: "passionate", Category: "sexual"},
	{Word: "genitalia", Replacement: "figure", Category: "sexual"},
	{Word: "topless", Replacement: "loosely dressed", Category: "sexual"},

	// ─── English: Contextual (weapons — allow in cinematic context) ─────────────
	// Only replaced when paired with graphic context; standalone is kept.
	{Word: "snuff film", Replacement: "dark cinema", Category: "violence"},
	{Word: "child abuse", Replacement: "traumatic childhood", Category: "violence"},
	{Word: "self-harm", Replacement: "internal struggle", Category: "violence"},
	{Word: "suicide", Replacement: "desperate situation", Category: "violence"},

	// ─── Chinese: Violence ───────────────────────────────────────────────────────
	{Word: "血腥", Replacement: "鲜红色", Category: "cn_violence"},
	{Word: "暴力血", Replacement: "激烈冲突", Category: "cn_violence"},
	{Word: "残忍杀", Replacement: "激烈战斗", Category: "cn_violence"},
	{Word: "尸体", Replacement: "倒下的身影", Category: "cn_violence"},
	{Word: "杀戮", Replacement: "战斗场面", Category: "cn_violence"},
	{Word: "凌辱", Replacement: "对峙场景", Category: "cn_violence"},
	{Word: "屠杀", Replacement: "惨烈战场", Category: "cn_violence"},
	{Word: "自残", Replacement: "内心挣扎", Category: "cn_violence"},
	{Word: "自杀", Replacement: "危险处境", Category: "cn_violence"},
	{Word: "割喉", Replacement: "近身搏斗", Category: "cn_violence"},

	// ─── Chinese: Sexual ─────────────────────────────────────────────────────────
	{Word: "裸体", Replacement: "着飘逸服饰", Category: "cn_sexual"},
	{Word: "色情", Replacement: "情感表达", Category: "cn_sexual"},
	{Word: "淫", Replacement: "情感", Category: "cn_sexual"},
	{Word: "性爱", Replacement: "亲密接触", Category: "cn_sexual"},
	{Word: "赤裸", Replacement: "轻薄衣物", Category: "cn_sexual"},

	// ─── Chinese: Platform-sensitive ─────────────────────────────────────────────
	{Word: "吸毒", Replacement: "神志不清的状态", Category: "cn_sensitive"},
	{Word: "毒品", Replacement: "禁忌物品", Category: "cn_sensitive"},
	{Word: "赌博", Replacement: "冒险游戏", Category: "cn_sensitive"},
	{Word: "黄赌毒", Replacement: "禁忌场所", Category: "cn_sensitive"},
}
