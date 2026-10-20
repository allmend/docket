-- Every refresh token descends from one login; family_id ties the chain
-- together so logout can revoke all of it, and a rotated token replayed after
-- its grace window (the classic theft signal) can kill the whole chain.
--
-- The default keeps the previous release's code working during a rolling
-- update: its inserts don't name family_id and simply start a family of one.
-- Existing rows each become their own family.
ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS family_id UUID NOT NULL DEFAULT gen_random_uuid();

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens (family_id);
