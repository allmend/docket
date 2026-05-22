ALTER TABLE retro_cards ADD COLUMN parent_id UUID REFERENCES retro_cards(id) ON DELETE SET NULL;
CREATE INDEX idx_retro_cards_parent ON retro_cards(org_id, parent_id) WHERE parent_id IS NOT NULL;
