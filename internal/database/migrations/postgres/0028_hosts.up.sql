-- 0028: Agent device monitoring - Host domain, credentials, metrics.
-- Agent is optional: resources.host_id is nullable and no core path depends on it.
CREATE TABLE IF NOT EXISTS hosts (
    id            TEXT PRIMARY KEY,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    name          TEXT NOT NULL,
    os            TEXT,
    agent_version TEXT,
    last_seen_at  TIMESTAMPTZ,
    last_cpu_pct  DOUBLE PRECISION,
    last_mem_pct  DOUBLE PRECISION,
    last_disk_pct DOUBLE PRECISION,
    last_net_in   BIGINT,
    last_net_out  BIGINT,
    last_disks    JSONB
);
CREATE INDEX IF NOT EXISTS idx_hosts_last_seen_at ON hosts(last_seen_at);

CREATE TABLE IF NOT EXISTS host_credentials (
    id           TEXT PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    host_id      TEXT NOT NULL,
    hash         TEXT NOT NULL,
    prefix       TEXT NOT NULL,
    is_active    BOOLEAN NOT NULL,
    last_used_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_host_credentials_hash ON host_credentials(hash);
CREATE INDEX IF NOT EXISTS idx_host_credentials_host_id ON host_credentials(host_id);

CREATE TABLE IF NOT EXISTS host_metrics (
    id         TEXT PRIMARY KEY,
    host_id    TEXT NOT NULL,
    sampled_at TIMESTAMPTZ NOT NULL,
    cpu_pct    DOUBLE PRECISION NOT NULL,
    mem_pct    DOUBLE PRECISION NOT NULL,
    net_in     BIGINT NOT NULL,
    net_out    BIGINT NOT NULL,
    disks      JSONB
);
CREATE INDEX IF NOT EXISTS idx_host_metrics_host_sampled ON host_metrics(host_id, sampled_at);

ALTER TABLE resources ADD COLUMN host_id TEXT;
CREATE INDEX IF NOT EXISTS idx_resources_host_id ON resources(host_id);
