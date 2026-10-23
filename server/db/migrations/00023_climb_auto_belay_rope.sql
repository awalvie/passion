-- +goose Up
-- On an auto belay with a rope trailing behind, to clip as if leading.
ALTER TABLE climb DROP CONSTRAINT climb_rope_style_check;
ALTER TABLE climb ADD CONSTRAINT climb_rope_style_check
    CHECK (rope_style IN ('lead', 'top_rope', 'auto_belay', 'auto_belay_rope', 'follow'));

-- +goose Down
UPDATE climb SET rope_style = 'auto_belay' WHERE rope_style = 'auto_belay_rope';
ALTER TABLE climb DROP CONSTRAINT climb_rope_style_check;
ALTER TABLE climb ADD CONSTRAINT climb_rope_style_check
    CHECK (rope_style IN ('lead', 'top_rope', 'auto_belay', 'follow'));
