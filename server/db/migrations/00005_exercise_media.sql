-- +goose Up
-- A private catalog can give one exercise several clips, or an image with no
-- video, so media is a list of {url, thumb_url}.
ALTER TABLE exercise
    DROP COLUMN video_url,
    DROP COLUMN thumbnail_url,
    ADD COLUMN media jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE exercise
    DROP COLUMN media,
    ADD COLUMN video_url text,
    ADD COLUMN thumbnail_url text;
