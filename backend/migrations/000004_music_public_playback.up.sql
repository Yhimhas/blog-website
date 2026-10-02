ALTER TABLE music_items ADD COLUMN public_playback_scope varchar(16) NOT NULL DEFAULT 'public' CHECK (public_playback_scope IN ('public','unknown'));
ALTER TABLE music_items ADD COLUMN public_media_kind varchar(16) NOT NULL DEFAULT 'unknown' CHECK (public_media_kind IN ('full','preview','unknown'));

CREATE TABLE music_playback_checks (
 item_id varchar(150) NOT NULL REFERENCES music_items(id),
 source_mode varchar(16) NOT NULL CHECK (source_mode IN ('public','account')),
 audience varchar(16) NOT NULL CHECK (audience IN ('public','private','blocked')),
 credential_version varchar(64) NOT NULL DEFAULT '',
 media_kind varchar(16) NOT NULL CHECK (media_kind IN ('full','preview','unknown')),
 error_code text NOT NULL DEFAULT '', checked_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(item_id,source_mode,audience,credential_version)
);

-- Fixed slots bound resource/rate state. Expired slots are overwritten, not deleted.
CREATE TABLE music_playback_control (
 kind varchar(64) NOT NULL, slot integer NOT NULL,
 key_hash varchar(64) NOT NULL, owner_hash varchar(64) NOT NULL DEFAULT '',
 used bigint NOT NULL DEFAULT 0, expires_at timestamptz NOT NULL,
 PRIMARY KEY(kind,slot)
);
CREATE INDEX music_playback_control_lookup ON music_playback_control(kind,key_hash,expires_at);
