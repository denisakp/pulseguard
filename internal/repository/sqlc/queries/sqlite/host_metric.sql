-- name: InsertHostMetric :exec
INSERT INTO host_metrics (
    id, host_id, sampled_at, cpu_pct, mem_pct, net_in, net_out, disks
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListHostMetricsInRange :many
SELECT * FROM host_metrics
WHERE host_id = ?1 AND sampled_at >= ?2 AND sampled_at <= ?3
ORDER BY sampled_at ASC;

-- name: DeleteHostMetricsOlderThan :execrows
DELETE FROM host_metrics WHERE sampled_at < ?1;

-- name: DeleteHostMetricsByHost :exec
DELETE FROM host_metrics WHERE host_id = ?;

-- name: DecimateHostMetrics :execrows
-- Keep at most one sample per (host, minute) for rows older than the cutoff,
-- deleting the rest. MIN(id) is the earliest ULID in each minute bucket;
-- substr(sampled_at,1,16) buckets by 'YYYY-MM-DD HH:MM'.
DELETE FROM host_metrics
WHERE host_metrics.sampled_at < ?1
  AND host_metrics.id NOT IN (
    SELECT MIN(hm.id) FROM host_metrics hm
    WHERE hm.sampled_at < ?1
    GROUP BY hm.host_id, substr(hm.sampled_at, 1, 16)
  );
