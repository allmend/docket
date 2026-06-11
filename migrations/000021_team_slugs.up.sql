ALTER TABLE teams ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';

-- Populate slug from name: lowercase, collapse non-alphanumeric runs to hyphens, trim edges
UPDATE teams
SET slug = trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'));

-- Disambiguate any duplicates within the same org by appending the lowercased key
UPDATE teams t
SET slug = t.slug || '-' || lower(t.key)
WHERE EXISTS (
    SELECT 1 FROM teams t2
    WHERE t2.org_id = t.org_id AND t2.slug = t.slug AND t2.id <> t.id
);

ALTER TABLE teams ALTER COLUMN slug SET NOT NULL;
ALTER TABLE teams ALTER COLUMN slug DROP DEFAULT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_teams_org_slug ON teams (org_id, slug);
