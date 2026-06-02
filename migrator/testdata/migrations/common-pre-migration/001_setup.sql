-- Pre-migration: Create tracking table for verification
-- Set search_path to test schema (matching sqldbtest.TestSchema)
SET search_path TO test, public;

CREATE TABLE IF NOT EXISTS migration_tracking (
    event TEXT NOT NULL,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO migration_tracking (event) VALUES ('pre-migration-executed');

