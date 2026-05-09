// Package stylepreset defines the canonical video/storyboard style preset keys
// and the mapping from legacy aliases. Single source of truth — update here only.
package stylepreset

import "strings"

// Canonical style keys used throughout the pipeline.
const (
Anime2D         = "anime-2d"
Anime3D         = "anime-3d"
LiveActionFilm  = "live-action-film"
LiveActionShort = "live-action-short"

// Default when no style is specified.
Default = Anime2D
)

// Canonical normalises a raw style_preset string (which may be a legacy alias,
// empty, or already canonical) into one of the four canonical keys above.
func Canonical(stylePreset string) string {
switch strings.TrimSpace(stylePreset) {
case "", Anime2D, "anime", "comic-dynamic", "guofeng-myth", "ink-poetry":
return Anime2D
case Anime3D, "fantasy-dream":
return Anime3D
case LiveActionFilm, "cinematic-epic", "vintage-film", "sci-fi-neon", "suspense-dark":
return LiveActionFilm
case LiveActionShort, "realistic-drama", "fashion-commercial", "documentary-natural", "urban-romance", "warm-healing":
return LiveActionShort
default:
return strings.TrimSpace(stylePreset)
}
}
