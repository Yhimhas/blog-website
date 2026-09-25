CREATE TABLE categories (id text PRIMARY KEY, slug varchar(100) UNIQUE NOT NULL, name varchar(160) NOT NULL);
CREATE TABLE tags (id text PRIMARY KEY, slug varchar(100) UNIQUE NOT NULL, name varchar(160) NOT NULL);
CREATE TABLE posts (
 id text PRIMARY KEY, slug varchar(100) UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
 title varchar(160) NOT NULL CHECK (length(btrim(title)) > 0), summary varchar(500) NOT NULL DEFAULT '',
 content_markdown text NOT NULL DEFAULT '' CHECK (octet_length(content_markdown) <= 204800),
 category_id text REFERENCES categories(id), status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
 published_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0), CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX posts_public_order ON posts(status,published_at DESC,id DESC);
CREATE TABLE post_tags (post_id text REFERENCES posts(id), tag_id text REFERENCES tags(id), PRIMARY KEY(post_id,tag_id));
CREATE TABLE admin_users (id text PRIMARY KEY, username varchar(100) UNIQUE NOT NULL, password_hash text NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE sessions (token_hash text PRIMARY KEY, admin_id text NOT NULL REFERENCES admin_users(id), expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE music_sources (
 id text PRIMARY KEY, provider text NOT NULL CHECK(provider IN ('netease','bilibili')), external_id text NOT NULL,
 title varchar(160) NOT NULL, source_url text NOT NULL, embed_url text, enabled boolean NOT NULL DEFAULT true,
 sync_status text NOT NULL DEFAULT 'pending' CHECK(sync_status IN ('pending','running','ready','failed')),
 synced_at timestamptz, last_error_code text, last_error_at timestamptz, snapshot_hash text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(provider,external_id)
);
CREATE TABLE music_items (
 id text PRIMARY KEY, provider text NOT NULL CHECK(provider IN ('netease','bilibili')), external_id text NOT NULL,
 part_key text NOT NULL DEFAULT '', title varchar(300) NOT NULL, author varchar(160), source_url text NOT NULL,
 duration_seconds integer CHECK(duration_seconds >= 0), availability text NOT NULL CHECK(availability IN ('available','unavailable','unknown')),
 embed_url text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(provider,external_id,part_key)
);
CREATE TABLE source_items (
 source_id text REFERENCES music_sources(id), item_id text REFERENCES music_items(id), position integer NOT NULL CHECK(position >= 0),
 active boolean NOT NULL DEFAULT true, first_seen timestamptz NOT NULL DEFAULT now(), last_seen timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(source_id,item_id)
);
CREATE INDEX source_items_order ON source_items(source_id,active,position,item_id);
CREATE TABLE netease_sync_runs (
 id text PRIMARY KEY, source_id text NOT NULL REFERENCES music_sources(id), status text NOT NULL CHECK(status IN ('running','ready','failed')),
 started_at timestamptz NOT NULL, finished_at timestamptz, candidate_count integer NOT NULL DEFAULT 0, error_code text, request_id text NOT NULL
);
CREATE TABLE daily_recommendations (date date PRIMARY KEY, timezone text NOT NULL, algorithm text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE daily_recommendation_items (date date REFERENCES daily_recommendations(date), position integer NOT NULL, item_id text REFERENCES music_items(id), snapshot jsonb NOT NULL, PRIMARY KEY(date,position), UNIQUE(date,item_id));
INSERT INTO music_sources(id,provider,external_id,title,source_url,embed_url) VALUES
 ('netease:595975585','netease','595975585','我喜欢的音乐','https://music.163.com/m/playlist?id=595975585&creatorId=417762656','https://music.163.com/outchain/player?type=0&id=595975585&auto=0&height=430');
