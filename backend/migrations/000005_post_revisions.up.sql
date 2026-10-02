CREATE TABLE post_revisions (
 post_id text PRIMARY KEY REFERENCES posts(id),
 title varchar(160) NOT NULL CHECK (length(btrim(title)) > 0),
 summary varchar(500) NOT NULL DEFAULT '',
 content_markdown text NOT NULL DEFAULT '' CHECK (octet_length(content_markdown) <= 204800),
 category_id text REFERENCES categories(id),
 updated_at timestamptz NOT NULL
);
CREATE TABLE post_revision_tags (
 post_id text REFERENCES post_revisions(post_id),
 tag_id text REFERENCES tags(id),
 PRIMARY KEY(post_id, tag_id)
);
