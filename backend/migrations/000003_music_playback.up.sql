ALTER TABLE music_items ADD COLUMN playback_checked_at timestamptz;
ALTER TABLE music_items ADD COLUMN playback_error_code text;
