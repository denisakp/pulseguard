DROP INDEX IF EXISTS idx_resources_host_id;
-- NOTE: SQLite has no "DROP COLUMN IF EXISTS"; the migration runner applies
-- every .sql file (up + down), so an unconditional DROP COLUMN would error when
-- host_id is absent. resources.host_id is additive/nullable and harmless when
-- the host tables are gone, so it is intentionally left in place on rollback.
DROP INDEX IF EXISTS idx_host_metrics_host_sampled;
DROP TABLE IF EXISTS host_metrics;
DROP INDEX IF EXISTS idx_host_credentials_host_id;
DROP INDEX IF EXISTS idx_host_credentials_hash;
DROP TABLE IF EXISTS host_credentials;
DROP INDEX IF EXISTS idx_hosts_last_seen_at;
DROP TABLE IF EXISTS hosts;
