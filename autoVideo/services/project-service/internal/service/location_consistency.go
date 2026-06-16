package service

import (
	"encoding/json"
	"strings"
	"unicode"
)

const (
	LocationViewExterior = "exterior"
	LocationViewInterior = "interior"
	LocationViewEntrance = "entrance"
	LocationViewAerial   = "aerial"
	LocationViewUnknown  = "unknown"
)

// LocationZoneProfile is a spatial partition under a location hub (e.g. bun shop interior).
type LocationZoneProfile struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Description   string `json:"description"`
	DescriptionEN string `json:"description_en,omitempty"`
	AssetID       *int64 `json:"asset_id,omitempty"`
	ViewType      string `json:"view_type,omitempty"`
}

// sceneAssetEntry indexes a scene-type asset for smart reference selection.
type sceneAssetEntry struct {
	Name        string
	ImageURL    string
	Hub         string
	Zone        string
	ViewType    string
	PanelImages []string
}

var locationHubSeparators = []string{"·", "・", "-", "—", "|", "/", "：", ":"}

var interiorKeywords = []string{
	"店内", "店里", "铺内", "室内", "内景", "屋内", "堂内", "后厨", "厨房", "卧室", "客厅", "办公室内",
	"inside", "interior", "indoor", " indoors",
}

var exteriorKeywords = []string{
	"门外", "门外侧", "店外", "室外", "外景", "街头", "街道", "门外街道", "门外台阶", "门外空地",
	"outside", "exterior", "outdoor", "street", "facade",
}

var entranceKeywords = []string{
	"门口", "门边", "门槛", "门内", "门外", "入口", "推门", "店门", "门楣", "threshold", "doorway", "entrance",
}

// NormalizeLocationViewType canonicalizes a view type token.
func NormalizeLocationViewType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case LocationViewExterior, "outdoor", "outside", "外景", "室外", "店外":
		return LocationViewExterior
	case LocationViewInterior, "indoor", "inside", "内景", "室内", "店内":
		return LocationViewInterior
	case LocationViewEntrance, "doorway", "threshold", "门口", "入口":
		return LocationViewEntrance
	case LocationViewAerial, "overhead", "bird", "俯视", "航拍":
		return LocationViewAerial
	default:
		return LocationViewUnknown
	}
}

// ParseLocationHubAndZone splits "刘师傅包子铺·内景" into hub + zone label.
func ParseLocationHubAndZone(location string) (hub, zone string) {
	trimmed := strings.TrimSpace(location)
	if trimmed == "" {
		return "", ""
	}
	for _, sep := range locationHubSeparators {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			left := strings.TrimSpace(trimmed[:idx])
			right := strings.TrimSpace(trimmed[idx+len(sep):])
			if left != "" && right != "" {
				return left, right
			}
		}
	}
	return trimmed, ""
}

// InferLocationViewType derives interior/exterior/entrance from storyboard text.
func InferLocationViewType(sceneDescription, location, shotType, explicitZone string) string {
	if vt := NormalizeLocationViewType(explicitZone); vt != LocationViewUnknown {
		return vt
	}
	if _, zone := ParseLocationHubAndZone(location); zone != "" {
		if vt := inferViewTypeFromZoneLabel(zone); vt != LocationViewUnknown {
			return vt
		}
	}
	combined := strings.ToLower(strings.TrimSpace(sceneDescription + " " + location))
	if containsAnyKeyword(combined, entranceKeywords) {
		return LocationViewEntrance
	}
	if containsAnyKeyword(combined, interiorKeywords) {
		return LocationViewInterior
	}
	if containsAnyKeyword(combined, exteriorKeywords) {
		return LocationViewExterior
	}
	switch strings.ToLower(strings.TrimSpace(shotType)) {
	case "establishing", "wide":
		if !containsAnyKeyword(combined, interiorKeywords) {
			return LocationViewExterior
		}
	}
	return LocationViewUnknown
}

func inferViewTypeFromZoneLabel(zone string) string {
	z := strings.ToLower(strings.TrimSpace(zone))
	switch {
	case strings.Contains(z, "内") || strings.Contains(z, "indoor") || strings.Contains(z, "interior"):
		return LocationViewInterior
	case strings.Contains(z, "外") || strings.Contains(z, "outdoor") || strings.Contains(z, "exterior") || strings.Contains(z, "street"):
		return LocationViewExterior
	case strings.Contains(z, "门口") || strings.Contains(z, "入口") || strings.Contains(z, "entrance") || strings.Contains(z, "door"):
		return LocationViewEntrance
	default:
		return LocationViewUnknown
	}
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, kw := range keywords {
		if kw != "" && strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// NormalizeLocationHub returns the parent location name without zone suffix.
func NormalizeLocationHub(location string) string {
	hub, _ := ParseLocationHubAndZone(location)
	return hub
}

func parseSceneAssetEntry(name, imageURL string, panelImages []string, metadata map[string]interface{}) sceneAssetEntry {
	hub, zone := ParseLocationHubAndZone(name)
	viewType := LocationViewUnknown
	if metadata != nil {
		if v, ok := metadata["view_type"].(string); ok {
			viewType = NormalizeLocationViewType(v)
		}
		if v, ok := metadata["location_hub"].(string); ok && strings.TrimSpace(v) != "" {
			hub = strings.TrimSpace(v)
		}
		if v, ok := metadata["location_zone"].(string); ok && strings.TrimSpace(v) != "" {
			zone = strings.TrimSpace(v)
		}
	}
	if viewType == LocationViewUnknown && zone != "" {
		viewType = inferViewTypeFromZoneLabel(zone)
	}
	if hub == "" {
		hub = strings.TrimSpace(name)
	}
	return sceneAssetEntry{
		Name:        strings.TrimSpace(name),
		ImageURL:    strings.TrimSpace(imageURL),
		Hub:         hub,
		Zone:        zone,
		ViewType:    viewType,
		PanelImages: panelImages,
	}
}

type sceneReferencePick struct {
	URL      string
	NoteZh   string
	Matched  bool
	Skipped  bool
	ViewType string
	Hub      string
}

// pickStoryboardSceneReference selects a scene asset image matching the requested view type.
func pickStoryboardSceneReference(
	requestedView, location string,
	sceneImages map[string]string,
	entries []sceneAssetEntry,
) sceneReferencePick {
	hub := NormalizeLocationHub(location)
	viewType := requestedView
	if viewType == "" || viewType == LocationViewUnknown {
		viewType = InferLocationViewType("", location, "", "")
	}
	pick := sceneReferencePick{ViewType: viewType, Hub: hub}

	lookupKey := strings.ToLower(strings.TrimSpace(location))
	if lookupKey != "" {
		if url := sceneImages[lookupKey]; url != "" {
			entryView := findSceneAssetViewType(entries, location)
			if entryView == LocationViewUnknown || viewType == LocationViewUnknown || viewsCompatible(entryView, viewType) {
				pick.URL = url
				pick.Matched = true
				pick.NoteZh = "已匹配场景资源：" + location
				return pick
			}
		}
	}
	if hub != "" {
		hubKey := strings.ToLower(hub)
		if url := sceneImages[hubKey]; url != "" {
			entryView := LocationViewUnknown
			for _, e := range entries {
				if strings.EqualFold(e.Hub, hub) && strings.EqualFold(e.Name, hub) {
					entryView = e.ViewType
					break
				}
			}
			if entryView == LocationViewUnknown || entryView == viewType || viewType == LocationViewUnknown || viewsCompatible(entryView, viewType) {
				pick.URL = url
				pick.Matched = true
				pick.NoteZh = "已匹配地点枢纽资源：" + hub
				return pick
			}
		}
	}

	var compatible []sceneAssetEntry
	for _, e := range entries {
		if e.ImageURL == "" {
			continue
		}
		if hub != "" && !strings.EqualFold(e.Hub, hub) {
			continue
		}
		if viewType == LocationViewUnknown || e.ViewType == LocationViewUnknown || viewsCompatible(e.ViewType, viewType) {
			compatible = append(compatible, e)
		}
	}
	if len(compatible) == 1 {
		pick.URL = compatible[0].ImageURL
		pick.Matched = true
		pick.NoteZh = "已匹配同地点场景资源：" + compatible[0].Name
		return pick
	}
	for _, e := range compatible {
		if e.ViewType == viewType {
			pick.URL = e.ImageURL
			pick.Matched = true
			pick.NoteZh = "已匹配同视角场景资源：" + e.Name
			return pick
		}
	}

	if viewType != LocationViewUnknown {
		pick.Skipped = true
		pick.NoteZh = "未找到与当前视角（" + viewTypeLabelZh(viewType) + "）一致的场景参考图，已跳过场景图参考，改用文字描述"
		return pick
	}
	return pick
}

func findSceneAssetViewType(entries []sceneAssetEntry, location string) string {
	lookup := strings.ToLower(strings.TrimSpace(location))
	for _, e := range entries {
		if strings.ToLower(strings.TrimSpace(e.Name)) == lookup {
			return e.ViewType
		}
	}
	hub := strings.ToLower(NormalizeLocationHub(location))
	for _, e := range entries {
		if strings.EqualFold(e.Hub, hub) && strings.EqualFold(e.Name, NormalizeLocationHub(location)) {
			return e.ViewType
		}
	}
	return LocationViewUnknown
}

func viewsCompatible(assetView, requestedView string) bool {
	if assetView == LocationViewUnknown || requestedView == LocationViewUnknown {
		return true
	}
	if assetView == requestedView {
		return true
	}
	if assetView == LocationViewEntrance && (requestedView == LocationViewInterior || requestedView == LocationViewExterior) {
		return true
	}
	if requestedView == LocationViewEntrance && (assetView == LocationViewInterior || assetView == LocationViewExterior) {
		return true
	}
	return false
}

func viewTypeLabelZh(viewType string) string {
	switch viewType {
	case LocationViewExterior:
		return "外景"
	case LocationViewInterior:
		return "内景"
	case LocationViewEntrance:
		return "门口/过渡"
	case LocationViewAerial:
		return "俯视/航拍"
	default:
		return "未指定"
	}
}

type locationProfileIndex struct {
	HubDesc       map[string]string
	ZoneDesc      map[string]string
	SharedVisual  map[string]string
}

func buildLocationProfileIndex(kwLibJSON []byte) locationProfileIndex {
	idx := locationProfileIndex{
		HubDesc:      map[string]string{},
		ZoneDesc:     map[string]string{},
		SharedVisual: map[string]string{},
	}
	if len(kwLibJSON) == 0 {
		return idx
	}
	var lib struct {
		LocationProfiles []struct {
			Name          string                `json:"name"`
			Description   string                `json:"description"`
			DescriptionEN string                `json:"description_en"`
			SharedVisual  string                `json:"shared_visual"`
			Zones         []LocationZoneProfile `json:"zones"`
		} `json:"location_profiles"`
	}
	if err := jsonUnmarshalKeywordLibrary(kwLibJSON, &lib); err != nil || len(lib.LocationProfiles) == 0 {
		return idx
	}
	for _, p := range lib.LocationProfiles {
		hubKey := strings.ToLower(strings.TrimSpace(p.Name))
		if hubKey == "" {
			continue
		}
		desc := strings.TrimSpace(p.DescriptionEN)
		if desc == "" {
			desc = strings.TrimSpace(p.Description)
		}
		if desc != "" {
			idx.HubDesc[hubKey] = desc
		}
		if sv := strings.TrimSpace(p.SharedVisual); sv != "" {
			idx.SharedVisual[hubKey] = sv
		}
		for _, z := range p.Zones {
			zDesc := strings.TrimSpace(z.DescriptionEN)
			if zDesc == "" {
				zDesc = strings.TrimSpace(z.Description)
			}
			if zDesc == "" {
				continue
			}
			zoneKey := strings.ToLower(strings.TrimSpace(z.ID))
			if zoneKey == "" {
				zoneKey = strings.ToLower(strings.TrimSpace(z.Label))
			}
			if zoneKey != "" {
				idx.ZoneDesc[hubKey+"|"+zoneKey] = zDesc
			}
		}
	}
	return idx
}

func jsonUnmarshalKeywordLibrary(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func resolveLocationDescriptionForPrompt(location, locationZone, viewType string, idx locationProfileIndex) string {
	hub := NormalizeLocationHub(location)
	hubKey := strings.ToLower(hub)
	zoneToken := strings.ToLower(strings.TrimSpace(locationZone))
	if zoneToken == "" {
		_, zoneLabel := ParseLocationHubAndZone(location)
		zoneToken = strings.ToLower(strings.TrimSpace(zoneLabel))
	}
	if zoneToken != "" {
		if desc := idx.ZoneDesc[hubKey+"|"+zoneToken]; desc != "" {
			return desc
		}
	}
	if vt := NormalizeLocationViewType(viewType); vt != LocationViewUnknown {
		if desc := idx.ZoneDesc[hubKey+"|"+vt]; desc != "" {
			return desc
		}
	}
	if desc := idx.HubDesc[hubKey]; desc != "" {
		return desc
	}
	return ""
}

func resolveLocationSharedVisual(location string, idx locationProfileIndex) string {
	hubKey := strings.ToLower(NormalizeLocationHub(location))
	return idx.SharedVisual[hubKey]
}

func locationHubsMatch(a, b string) bool {
	ha := strings.ToLower(strings.TrimSpace(NormalizeLocationHub(a)))
	hb := strings.ToLower(strings.TrimSpace(NormalizeLocationHub(b)))
	return ha != "" && ha == hb
}

func sceneEntriesFromDirectAssets(refs []storyboardAssetReference) []sceneAssetEntry {
	var out []sceneAssetEntry
	for _, asset := range refs {
		t := strings.ToLower(strings.TrimSpace(asset.Type))
		if t != "scene" && t != "location" {
			continue
		}
		if strings.TrimSpace(asset.ImageURL) == "" {
			continue
		}
		out = append(out, parseSceneAssetEntry(asset.Name, asset.ImageURL, asset.PanelImages, nil))
	}
	return out
}

func isMostlyASCII(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	ascii := 0
	total := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if r < 128 {
			ascii++
		}
	}
	return total > 0 && float64(ascii)/float64(total) > 0.55
}
