DROP INDEX IF EXISTS idx_retro_cards_parent;
ALTER TABLE retro_cards DROP COLUMN IF EXISTS parent_id;
