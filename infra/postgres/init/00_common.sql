-- Shared trigger function: keeps `updated_at` current on every UPDATE.
-- Every table in this schema has created_at/updated_at bookkeeping
-- columns (with indexes), in addition to whatever domain-specific
-- timestamps it needs.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
