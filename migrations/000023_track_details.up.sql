-- Tracks (board tags) grow a description and an optional lead, per the
-- Settings - Tracks design. Lead is soft: removing the user clears it.
ALTER TABLE tags
  ADD COLUMN description TEXT NOT NULL DEFAULT '',
  ADD COLUMN lead_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
