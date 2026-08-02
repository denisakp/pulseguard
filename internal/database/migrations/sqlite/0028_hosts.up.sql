-- 0028: Agent device monitoring - Host domain, credentials, metrics.
-- Agent is optional: resources.host_id is nullable and no core path depends on it.
CREATE TABLE IF NOT EXISTS hosts (
    id            TEXT PRIMARY KEY,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL,
    name          TEXT NOT NULL,
    os            TEXT,
    agent_version TEXT,
    last_seen_at  DATETIME,
    last_cpu_pct  REAL,
    last_mem_pct  REAL,
    last_disk_pct REAL,
    last_net_in   INTEGER,
    last_net_out  INTEGER,
    last_disks    TEXT
);
CREATE INDEX IF NOT EXISTS idx_hosts_last_seen_at ON hosts(last_seen_at);

CREATE TABLE IF NOT EXISTS host_credentials (
    id           TEXT PRIMARY KEY,
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL,
    host_id      TEXT NOT NULL,
    hash         TEXT NOT NULL,
    prefix       TEXT NOT NULL,
    is_active    INTEGER NOT NULL,
    last_used_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_host_credentials_hash ON host_credentials(hash);
CREATE INDEX IF NOT EXISTS idx_host_credentials_host_id ON host_credentials(host_id);

CREATE TABLE IF NOT EXISTS host_metrics (
    id         TEXT PRIMARY KEY,
    host_id    TEXT NOT NULL,
    sampled_at DATETIME NOT NULL,
    cpu_pct    REAL NOT NULL,
    mem_pct    REAL NOT NULL,
    net_in     INTEGER NOT NULL,
    net_out    INTEGER NOT NULL,
    disks      TEXT
);
CREATE INDEX IF NOT EXISTS idx_host_metrics_host_sampled ON host_metrics(host_id, sampled_at);

ALTER TABLE resources ADD COLUMN host_id TEXT;
CREATE INDEX IF NOT EXISTS idx_resources_host_id ON resources(host_id);
