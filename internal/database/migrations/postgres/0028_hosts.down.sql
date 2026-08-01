DROP INDEX IF EXISTS idx_resources_host_id;
ALTER TABLE resources DROP COLUMN IF EXISTS host_id;
DROP INDEX IF EXISTS idx_host_metrics_host_sampled;
DROP TABLE IF EXISTS host_metrics;
DROP INDEX IF EXISTS idx_host_credentials_host_id;
DROP INDEX IF EXISTS idx_host_credentials_hash;
DROP TABLE IF EXISTS host_credentials;
DROP INDEX IF EXISTS idx_hosts_last_seen_at;
DROP TABLE IF EXISTS hosts;
