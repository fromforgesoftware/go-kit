-- Canonical outbox table shape. Each producer service copies this
-- into its migrations folder and adjusts the schema/table name to
-- match its own namespace (e.g. workspace.outbox, accounting.outbox).
--
-- The kit's outbox/postgres.Repository expects this exact column set;
-- adding extra columns is fine (they'll just be ignored) but renaming
-- or dropping the canonical ones will break the dispatcher.
--
-- Per-service tables (rather than one shared `outbox` schema):
--   1. write contention stays bounded to the service's traffic;
--   2. each service's migrator owns the GRANTs for its own role;
--   3. local DB transactions trivially cover Enqueue + business insert.
--
-- Replace WORKSPACE_SCHEMA / WORKSPACE_SERVICE with the producer
-- service's schema and runtime role name.

CREATE TABLE WORKSPACE_SCHEMA.outbox (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind        TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts    INTEGER     NOT NULL DEFAULT 0,
    -- retry_at is the next time Claim is allowed to pick this row up.
    -- Defaulted to NOW() so new rows are immediately eligible.
    retry_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error  TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'pending'
);

-- The hot index: drainers scan pending rows in retry_at order.
CREATE INDEX idx_outbox_pending_retry
    ON WORKSPACE_SCHEMA.outbox (retry_at)
    WHERE status = 'pending';

GRANT USAGE ON SCHEMA WORKSPACE_SCHEMA TO WORKSPACE_SERVICE;
GRANT SELECT, INSERT, UPDATE, DELETE ON WORKSPACE_SCHEMA.outbox TO WORKSPACE_SERVICE;
