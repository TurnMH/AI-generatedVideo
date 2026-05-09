"-- Migration: Clean visual tone from asset descriptions
-- This removes the old visual tone suffix that was stored in description field
-- Before: '少年男子...（视觉基调：时代（明朝）...）'
-- After: '少年男子...' (visual tone is now applied dynamically)

-- PostgreSQL version
UPDATE assets
SET description = TRIM(
    REGEXP_REPLACE(
        description,
        '[\n\r]+\s*视觉基调：.*$',
        '',
        'g'
    )
)
WHERE description ~ '[\n\r]+\s*视觉基调：';

-- Also clean any remaining inline visual tone references
UPDATE assets
SET description = TRIM(
    REGEXP_REPLACE(
        description,
        '。?\s*（视觉基调：[^）]+）',
        '',
        'g'
    )
)
WHERE description ~ '（视觉基调：';
"