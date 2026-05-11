# Prompt Governance

This document draws the line between prompts that are safe to operate from the database and prompts that must stay hardcoded in services.

## Operable In Database

These prompts are content-first and can be iterated by operators without changing service logic.

- `script-service.prompt_templates`
- Resource scope: storyboard image prompt templates
- Current style keys seeded by migrations:
  - `storyboard_anime2d`
  - `storyboard_anime3d`
  - `storyboard_cinematic`
  - `storyboard_live_action`

Operational fields:

- `style_key`: the style preset key used by project-service
- `resource_type`: currently `storyboard`
- `model_binding`: optional model-specific override
- `version`: operator-visible template version label
- `content`: the actual prompt shell with placeholders like `{scene}` `{characters}` `{action}` `{mood}`

Why this belongs in DB:

- The template is a prompt shell, not a control-flow contract.
- Operators may AB-test wording without rebuilding services.
- project-service already fetches it dynamically through `/api/v1/prompt-templates`.

## Must Stay Hardcoded

These prompts encode output contracts, safety constraints, or downstream assumptions. They should remain in code.

- Asset prompts for `character` / `scene` / `prop`
  - `character-service/internal/service/asset_service.go`
- Script preprocessing and scene splitting prompts
  - `project-service/internal/service/episode_service.go`
- Continuity audit prompts
  - `project-service/internal/service/scene_continuity_auditor.go`
- Video motion prompts and continuity laws
  - `video-service/internal/service/motion_prompt_service.go`

Why these stay in code:

- They are tightly coupled with JSON schema, extraction rules, and downstream parsing.
- They carry strict negative constraints such as no-person scene assets and identity-lock rules.
- A freeform DB edit here can silently break parsing, continuity, or generation safety.

## Mixed Ownership

Storyboard prompt generation is mixed.

- DB provides the outer prompt shell via `prompt_templates.content`.
- Code still owns:
  - placeholder substitution
  - scene description enrichment
  - continuity context
  - fallback prompt composition when no template is available

This boundary keeps operations flexible without letting a DB edit break the generation pipeline.