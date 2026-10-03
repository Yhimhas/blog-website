-- A transactional outbox reduced to a monotonic public-content revision.
-- Initial revision forces a first build, including on an existing database.
CREATE TABLE seo_publication (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 source_id uuid NOT NULL DEFAULT gen_random_uuid(),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 applied_revision bigint NOT NULL DEFAULT 0 CHECK (applied_revision >= 0 AND applied_revision <= revision),
 last_attempt_at timestamptz,
 published_at timestamptz,
 last_error text NOT NULL DEFAULT ''
);
INSERT INTO seo_publication(singleton) VALUES (true);

-- Runs inside the article transaction: failed writes roll back the event too.
-- Version-only updates (saving a published revision) do not affect public SEO.
CREATE FUNCTION mark_seo_publication() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'INSERT' THEN
  IF NEW.status <> 'published' OR NEW.published_at IS NULL THEN RETURN NULL; END IF;
 ELSIF TG_OP = 'DELETE' THEN
  IF OLD.status <> 'published' OR OLD.published_at IS NULL THEN RETURN NULL; END IF;
 ELSE
  IF (OLD.status <> 'published' OR OLD.published_at IS NULL)
     AND (NEW.status <> 'published' OR NEW.published_at IS NULL) THEN RETURN NULL; END IF;
  IF ROW(OLD.slug, OLD.title, OLD.summary, OLD.content_markdown, OLD.status, OLD.published_at, OLD.updated_at)
     IS NOT DISTINCT FROM
     ROW(NEW.slug, NEW.title, NEW.summary, NEW.content_markdown, NEW.status, NEW.published_at, NEW.updated_at)
     THEN RETURN NULL; END IF;
 END IF;
 UPDATE seo_publication SET revision = revision + 1 WHERE singleton = true;
 IF NOT FOUND THEN RAISE EXCEPTION 'seo_publication singleton missing'; END IF;
 RETURN NULL;
END;
$$;
CREATE TRIGGER posts_seo_publication AFTER INSERT OR UPDATE OR DELETE ON posts
 FOR EACH ROW EXECUTE FUNCTION mark_seo_publication();
