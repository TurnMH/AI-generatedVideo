-- Add video_prompt to storyboards for pre-computed English video generation prompts.
-- This field is reserved for future LLM-based translation of scene_description
-- into an optimized English video prompt, eliminating runtime parsing.
ALTER TABLE storyboards ADD COLUMN IF NOT EXISTS video_prompt TEXT NOT NULL DEFAULT '';
