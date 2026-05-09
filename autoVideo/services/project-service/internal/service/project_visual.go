package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/stylepreset"
)

func decodeStoryboardConfig(project *model.Project) map[string]interface{} {
	if project == nil || len(project.StoryboardConfig) == 0 {
		return nil
	}
	var storyboardConfig map[string]interface{}
	_ = json.Unmarshal(project.StoryboardConfig, &storyboardConfig)
	return storyboardConfig
}

func buildProjectVisualPrompt(project *model.Project) string {
	if project == nil {
		return ""
	}
	storyboardConfig := decodeStoryboardConfig(project)
	stylePreset := strings.TrimSpace(asString(storyboardConfig["style_preset"]))
	parts := make([]string, 0, 5)
	// Live-action context (region/era/ethnicity) goes FIRST so diffusion models weight
	// these highest — they define the dominant visual language of the entire storyboard.
	// Applied to all style presets (including anime-2d/anime-3d) so period/regional
	// anime projects (e.g. Tang-dynasty anime) also receive the era constraints that
	// were injected into individual character assets.
	_ = stylepreset.Canonical(stylePreset)
	if cue := storyboardLiveActionContextCue(
		asString(storyboardConfig["region"]),
		asString(storyboardConfig["era"]),
		asString(storyboardConfig["ethnicity"]),
	); cue != "" {
		parts = append(parts, cue)
	}
	if cue := storyboardStyleCue(stylePreset); cue != "" {
		parts = append(parts, cue)
	}
	for _, tag := range project.StyleTags {
		if cue := storyboardStyleTagCue(tag); cue != "" {
			parts = append(parts, cue)
		}
	}
	if cue := storyboardMotionCue(strings.TrimSpace(asString(storyboardConfig["motion_mode"]))); cue != "" {
		parts = append(parts, cue)
	}
	parts = dedupeStoryboardVisualCues(parts)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func storyboardStylePreset(project *model.Project) string {
	return stylepreset.Canonical(strings.TrimSpace(asString(decodeStoryboardConfig(project)["style_preset"])))
}

func buildStoryboardNegativePrompt(project *model.Project) string {
	parts := []string{
		"text",
		"watermark",
		"logo",
		"subtitle",
		"speech bubble",
		"comic panel grid",
		"split screen",
		"collage",
		"blurry",
		"low detail",
		"duplicate subject",
		"bad anatomy",
		"deformed hands",
		"extra fingers",
		"extra limbs",
		"cropped face",
	}

	switch storyboardStylePreset(project) {
	case "anime-2d", "anime-3d":
		parts = append(parts,
			"photorealistic skin",
			"live-action photography",
			"raw camera snapshot",
		)
	case "live-action-film", "live-action-short":
		parts = append(parts,
			"anime",
			"cartoon",
			"illustration",
			"cel shading",
			"chibi",
			"plastic CGI look",
		)
	}

	return strings.Join(dedupeStoryboardVisualCues(parts), ", ")
}

// appendModelNegativeTokens adds model-specific negative prompt tokens to improve
// diffusion model output quality (T4). DALL-E ignores negative prompts so returns empty.
func appendModelNegativeTokens(baseNeg, modelName string) string {
	lm := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.Contains(lm, "sdxl") || strings.Contains(lm, "comfyui") || strings.Contains(lm, "sd"):
		extras := []string{
			"low quality", "worst quality", "normal quality", "jpeg artifacts",
			"noise", "grain", "out of focus", "overexposed", "underexposed",
			"poorly drawn", "ugly", "mutated", "disfigured",
		}
		if baseNeg == "" {
			return strings.Join(extras, ", ")
		}
		return baseNeg + ", " + strings.Join(extras, ", ")
	case strings.Contains(lm, "flux"):
		extras := []string{"low quality", "blurry", "out of focus", "disfigured"}
		if baseNeg == "" {
			return strings.Join(extras, ", ")
		}
		return baseNeg + ", " + strings.Join(extras, ", ")
	default:
		// DALL-E and unknown models — return base as-is.
		return baseNeg
	}
}

// appendNoPeopleNegativeTokens adds anti-human-figure tokens to the negative prompt,
// used for environment/prop-only storyboards that must not contain incidental people.
func appendNoPeopleNegativeTokens(baseNeg string) string {
	extras := []string{"people", "human figures", "persons", "faces", "portraits", "characters", "crowd"}
	if strings.TrimSpace(baseNeg) == "" {
		return strings.Join(extras, ", ")
	}
	return baseNeg + ", " + strings.Join(extras, ", ")
}

func appendProjectVisualPrompt(prompt, visualPrompt string) string {
	trimmedPrompt := strings.TrimSpace(prompt)
	trimmedVisual := strings.TrimSpace(visualPrompt)
	if trimmedVisual == "" || strings.Contains(trimmedPrompt, trimmedVisual) {
		return trimmedPrompt
	}
	if trimmedPrompt == "" {
		return "Project visual direction: " + trimmedVisual
	}
	return trimmedPrompt + " Project visual direction: " + trimmedVisual
}

func dedupeStoryboardVisualCues(parts []string) []string {
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func asString(value interface{}) string {
	switch current := value.(type) {
	case string:
		return current
	default:
		return ""
	}
}

func storyboardStyleCue(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d":
		return "2D anime storytelling, clean line art, illustrated character styling, cel-shaded color blocking."
	case "anime-3d":
		return "3D anime look, dimensional characters, stylized materials, toon-shaded depth and CG staging."
	case "live-action-film":
		return "live-action film look, realistic environments, dramatic cinematic lighting, avoid anime/cartoon stylization."
	case "live-action-short":
		return "live-action short drama, close-up performance, grounded real-world sets, natural skin texture, avoid anime/cartoon stylization."
	default:
		return ""
	}
}

func storyboardStyleTagCue(tag string) string {
	switch strings.TrimSpace(tag) {
	case "写实":
		return "realistic materials, natural texture detail, grounded visual treatment."
	case "纪录片":
		return "documentary tone, natural light, real-world observation."
	case "广告感":
		return "commercial polish, premium lighting, refined hero composition."
	case "欧美电影感", "美剧感":
		return "cinematic live-action framing, authentic locations, layered production lighting."
	case "都市夜景":
		return "real city night lighting, practical neon, grounded urban atmosphere."
	case "复古胶片":
		return "film grain, vintage color temperature, retro production design."
	default:
		return ""
	}
}

func storyboardMotionCue(motionMode string) string {
	switch motionMode {
	case "gentle":
		return "gentle restrained camera language and softer emotional pacing."
	case "dynamic":
		return "dynamic energy, clearer motion direction, stronger action tension."
	case "cinematic":
		return "cinematic pacing, theatrical framing, richer shot scale contrast."
	default:
		return ""
	}
}

// storyboardLiveActionContextCue builds a rich English context cue from optional
// live-action reference fields stored in storyboard_config.
// Each field is expanded into detailed visual descriptors so that image generators
// produce culturally accurate environments, costumes, and character appearances.
func storyboardLiveActionContextCue(region, era, ethnicity string) string {
	region = strings.TrimSpace(region)
	era = strings.TrimSpace(era)
	ethnicity = strings.TrimSpace(ethnicity)
	if region == "" && era == "" && ethnicity == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if region != "" {
		parts = append(parts, expandRegionVisualCue(region))
	}
	if era != "" {
		parts = append(parts, expandEraVisualCue(era))
	}
	if ethnicity != "" {
		parts = append(parts, expandEthnicityVisualCue(ethnicity))
	}
	return strings.Join(parts, " ")
}

// hasVisualKeyword reports whether s (already lower-cased) contains any of subs.
func hasVisualKeyword(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// expandRegionVisualCue converts a free-text region/country value into a rich
// English visual descriptor covering architecture, environment, and cultural props.
func expandRegionVisualCue(region string) string {
	lower := strings.ToLower(region)
	switch {
	case hasVisualKeyword(lower, "古代中国", "ancient china"):
		return "Setting: ancient Chinese imperial environment — towering timber-frame palace halls with vermilion columns, carved stone archways, stone-paved inner courtyards, red silk banners, hanging lanterns, period wooden furniture and bronze vessels."
	case hasVisualKeyword(lower, "中国", "china", "chinese"):
		return "Setting: authentic Chinese environment — traditional architecture with upturned glazed-tile roof eaves, red lacquer columns, circular moon gates, stone courtyard pavilions, hanging silk lanterns; or modern Chinese cityscape with high-density towers, LED neon Chinese-character signage, bustling street markets."
	case hasVisualKeyword(lower, "日本", "japan", "japanese"):
		return "Setting: authentic Japanese environment — wooden torii shrine gates, raked stone gardens, shoji paper screens, tatami-matted interiors, cherry blossom canopies; or modern Japanese urban district with convenience store signage, vending machines, dense neon multilevel shopfront facades."
	case hasVisualKeyword(lower, "韩国", "korea", "korean"):
		return "Setting: authentic Korean environment — hanok timber-frame village with curved clay-tile rooftops, celadon ceramic vessels, hanji paper lanterns; or modern Korean urban setting with Hangul neon signage, dense apartment towers, pojangmacha street food tent stalls."
	case hasVisualKeyword(lower, "英国", "england", "britain", "british", "uk"):
		return "Setting: British environment — Victorian red-brick terraced housing, cast-iron gas lampposts, grey overcast skies, Georgian stone facades, rainy cobblestone high streets, traditional pub signage, double-decker buses."
	case hasVisualKeyword(lower, "法国", "france", "paris", "french"):
		return "Setting: French Parisian environment — Haussmann cream-stone apartment blocks with wrought-iron Juliet balconies, mosaic-tiled cafe floors, tree-lined boulevards, warm golden ambient light, flower and produce market stalls."
	case hasVisualKeyword(lower, "欧洲", "europe", "european"):
		return "Setting: European environment — Gothic or Baroque stone architecture, ornate cathedral spires, cobblestone city plazas, wrought-iron balconies, warm limestone facades, cafe-lined boulevards."
	case hasVisualKeyword(lower, "美国", "america", "american", "usa"):
		return "Setting: American environment — wide tree-lined suburban streets with clapboard or brick houses, chrome retro diners, downtown skyscraper canyons, neon roadside motel signs, vast interstate highways with billboard advertising."
	case hasVisualKeyword(lower, "印度", "india", "indian"):
		return "Setting: Indian environment — ornate Mughal or Dravidian stone architecture with intricate carved facades, vibrant spice and textile bazaars with saturated color, autorickshaws, warm tropical golden light, incense smoke haze."
	case hasVisualKeyword(lower, "中东", "middle east", "arab", "阿拉伯"):
		return "Setting: Middle Eastern environment — Islamic geometric tile mosaic architecture, horseshoe arches, sand-toned mudbrick walls, date palms and desert landscape, souk bazaar with brass lanterns and patterned textiles."
	case hasVisualKeyword(lower, "非洲", "africa", "african"):
		return "Setting: African environment — terracotta-hued earthen architecture, savanna grassland with wide acacia trees and dust-gold light; or vibrant West African urban market with saturated patterned fabric stalls and dense street commerce."
	case hasVisualKeyword(lower, "古罗马", "roman", "rome"):
		return "Setting: ancient Roman environment — white marble columns and triumphal arches, tessellated mosaic courtyard floors, colonnaded forum spaces, red-tiled villa rooftops, oil-lamp sconces, togas and draped tunics."
	case hasVisualKeyword(lower, "古希腊", "greek", "greece"):
		return "Setting: ancient Greek environment — white limestone Doric temple architecture, sun-bleached stone plazas overlooking the Aegean, olive groves, amphitheater stone seating, draped chiton garments."
	case hasVisualKeyword(lower, "东南亚", "southeast asia", "thailand", "泰国", "越南", "vietnam"):
		return "Setting: Southeast Asian tropical environment — gilded Buddhist temple spires with tiered finials, wooden stilt houses over water, lush jungle canopy, floating market boats, vibrant orchid and lotus floral motifs."
	case hasVisualKeyword(lower, "俄罗斯", "russia", "russian"):
		return "Setting: Russian environment — onion-dome cathedral silhouettes, Soviet constructivist concrete blocks, birch forest under grey winter skies, wooden dacha cottages, traditional folk embroidery geometric patterns."
	default:
		return fmt.Sprintf("Setting: %s environment with culturally authentic architecture, indigenous building materials, region-specific environmental details, and props that unmistakably communicate a %s location.", region, region)
	}
}

// expandEraVisualCue converts a free-text era/dynasty value into a detailed English
// descriptor covering period-accurate clothing, architecture, props, and atmosphere.
func expandEraVisualCue(era string) string {
	lower := strings.ToLower(era)
	switch {
	case hasVisualKeyword(lower, "唐朝", "唐代", "tang dynasty", "tang"):
		return "Tang Dynasty era (618–907 AD): men in round-collar wide-sleeve pao robes (圆领袍衫) with puku headdress (幞头) and cloth boots; women in chest-high ruqun with pibo sashes and elaborate high-bun hairstyles with golden hairpins; clothing must strictly follow each character's gender — do not apply female garments to male characters; palatial eave halls, Silk Road cosmopolitan market atmosphere with exotic goods and diverse traders."
	case hasVisualKeyword(lower, "宋朝", "宋代", "song dynasty"):
		return "Song Dynasty era (960–1279 AD): refined scholar-court aesthetics, understated silk garments in muted celadon and ink-wash tones, civilian market prosperity, Song porcelain and ink-wash painting motifs as background props, delicate wooden scholar furniture."
	case hasVisualKeyword(lower, "元朝", "元代", "yuan dynasty"):
		return "Yuan Dynasty era (1271–1368 AD): Mongolian court culture, deel and fur-trimmed robes, horse culture, nomadic tent-palace architecture blending Chinese timber halls with Central Asian felt-yurt design elements."
	case hasVisualKeyword(lower, "明朝", "明代", "ming dynasty", "ming"):
		return "Ming Dynasty era (1368–1644 AD): layered hanfu court robes with wide embroidered sleeves and jade belt accessories, elaborate pearl and gold hairpins, massive red-lacquered timber palatial halls with carved imperial gates, blue-and-white porcelain vessels, silk brocade textiles."
	case hasVisualKeyword(lower, "清朝", "清代", "qing dynasty", "qing"):
		return "Qing Dynasty era (1644–1912 AD): embroidered qipao and changshan court robes, queue braided hairstyles, ornate Manchu headdresses with jade and coral, lacquered screen partitions, carved rosewood furniture, imperial yellow glazed-tile rooftops."
	case hasVisualKeyword(lower, "民国", "republic of china"):
		return "Republican-era China (1912–1949): modernised stand-collar qipao in silk alongside Western-influenced suits, transitional urban architecture blending Chinese upturned rooflines with Art Deco facades, rickshaws and early automobiles sharing the same cobblestone street."
	case hasVisualKeyword(lower, "战国", "warring states"):
		return "Warring States period (475–221 BC): lacquered leather armor with bronze plate fittings, deep-sleeved cross-collar pao robes, bronze ritual vessels, walled city fortifications with wooden watchtowers, war chariot silhouettes."
	case hasVisualKeyword(lower, "秦朝", "秦代", "qin dynasty", "qin"):
		return "Qin Dynasty era (221–206 BC): standardised bronze armor echoing the terracotta warrior aesthetic, rammed-earth city walls, grand stone ceremonial roads, imperial banners and bronze seals."
	case hasVisualKeyword(lower, "汉朝", "汉代", "han dynasty", "han"):
		return "Han Dynasty era (206 BC–220 AD): silk hanfu robes with long trailing sleeves, lacquered wooden vessels, bronze incense burners, tile-roofed earthen compounds, silk road trade goods (spices, silks, glassware) as background props."
	case hasVisualKeyword(lower, "中世纪", "medieval", "middle ages"):
		return "Medieval era (5th–15th century): stone castle fortifications with crenellated battlements, chain mail and plate armor, candlelit great halls with tapestry wall hangings, thatched peasant villages, heraldic banner-draped tournament grounds."
	case hasVisualKeyword(lower, "维多利亚", "victorian"):
		return "Victorian era (1837–1901): corseted bustle gowns and frock coats, horse-drawn carriages on gas-lit cobblestone streets, iron-and-glass Victorian industrial architecture, dense factory smokestacks on the skyline, ornate floral wallpapered interiors."
	case hasVisualKeyword(lower, "二战", "world war", "ww2", "1940s", "四十年代"):
		return "World War II era (1940s): period military uniforms with accurate insignia, rationing-era civilian clothing (utility dress, double-breasted suits), bomb-damaged urban rubble environments, propaganda poster signage, vintage chrome-bumper automobiles."
	case hasVisualKeyword(lower, "1990", "九十年代", "90s"):
		return "1990s era: high-waist denim, oversized windbreaker jackets, CRT televisions and early personal computers, older sedan automobiles with chrome bumpers, VHS rental store aesthetic, fluorescent tube-lit interiors."
	case hasVisualKeyword(lower, "1980", "八十年代", "80s"):
		return "1980s era: neon-colored athletic wear, geometric-pattern knitwear, boombox radios, tube televisions, bold synthetic fabrics, arcade game environment, synth-pop cultural atmosphere."
	case hasVisualKeyword(lower, "未来", "future", "futuristic", "sci-fi", "科幻"):
		return "Futuristic setting: sleek matte and brushed-chrome architecture, holographic floating interface displays, advanced personal wearable technology, high-contrast neon-lit night cityscapes, engineered materials with geometric precision."
	case hasVisualKeyword(lower, "现代", "modern", "contemporary", "当代", "现在"):
		return "Contemporary modern era: current-season fashion, ubiquitous smartphones and digital display screens, modern glass-and-steel or concrete architecture, electric vehicles, global urban consumer culture."
	case hasVisualKeyword(lower, "古代", "ancient", "古典", "classical"):
		return "Ancient classical period: hand-crafted pre-industrial architecture, period-accurate hand-sewn garments, oil-lamp and candle lighting, non-motorised transport, historically authentic material culture and craft objects."
	default:
		return fmt.Sprintf("Era: %s — period-accurate clothing silhouettes, hairstyles, architecture style, props, and technology level all consistent with the %s period; no anachronistic modern elements.", era, era)
	}
}

// expandEthnicityVisualCue converts a free-text ethnicity value into a focused
// English descriptor covering facial features, skin tone, hair, and styling.
func expandEthnicityVisualCue(ethnicity string) string {
	lower := strings.ToLower(ethnicity)
	switch {
	case hasVisualKeyword(lower, "汉族", "汉人", "中国人", "东亚人", "east asian", "chinese"):
		return "Characters: East Asian / Chinese appearance — straight black hair, warm ivory to light tan complexion, almond-shaped eyes, refined East Asian facial proportions, culturally appropriate Chinese styling."
	case hasVisualKeyword(lower, "日本人", "japanese"):
		return "Characters: Japanese appearance — straight black or dark-brown hair, light to medium fair complexion, refined East Asian facial features, Japanese cultural styling cues."
	case hasVisualKeyword(lower, "韩国人", "korean"):
		return "Characters: Korean appearance — straight black hair, fair to light complexion, smooth skin, contemporary Korean fashion styling and grooming."
	case hasVisualKeyword(lower, "白人", "欧洲人", "caucasian", "european", "western"):
		return "Characters: Caucasian / European appearance — varied hair from fair blond to dark brown, light complexion with warm or cool undertones, European facial bone structure."
	case hasVisualKeyword(lower, "非洲人", "黑人", "african", "black"):
		return "Characters: African / Black appearance — rich dark melanin complexion, natural hair textures from tight coil to loose curl, strong bone structure, African facial proportions."
	case hasVisualKeyword(lower, "印度人", "南亚", "indian", "south asian"):
		return "Characters: South Asian / Indian appearance — warm medium to deep brown complexion, dark black hair, expressive dark eyes, South Asian facial structure."
	case hasVisualKeyword(lower, "中东", "阿拉伯", "middle eastern", "arab"):
		return "Characters: Middle Eastern / Arab appearance — olive to warm tan complexion, dark expressive eyes, dark hair, strong angular facial features."
	case hasVisualKeyword(lower, "拉丁", "latino", "hispanic", "latin"):
		return "Characters: Latino / Hispanic appearance — warm olive to medium-brown complexion, dark hair, warm skin undertone, Latin American facial proportions."
	case hasVisualKeyword(lower, "东南亚", "southeast asian"):
		return "Characters: Southeast Asian appearance — warm tan to medium-brown complexion, dark black hair, dark eyes, Southeast Asian facial structure."
	case hasVisualKeyword(lower, "少数民族", "民族", "ethnic minority"):
		return "Characters: ethnic minority appearance with culturally distinctive facial features, traditional ethnic costume and accessories appropriate to their specific heritage."
	default:
		return fmt.Sprintf("Characters: %s appearance with ethnically authentic facial features, skin tone, hair type, and culturally appropriate styling consistent with %s heritage.", ethnicity, ethnicity)
	}
}
