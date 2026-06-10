-- name: UpsertScanResult :one
INSERT INTO scan_results (
    package_version_id, sha256, status,
    total_findings, critical_findings, high_findings, medium_findings, low_findings,
    highest_severity, findings_json, error_message, scanned_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
ON CONFLICT (package_version_id) DO UPDATE
    SET sha256             = EXCLUDED.sha256,
        status             = EXCLUDED.status,
        total_findings     = EXCLUDED.total_findings,
        critical_findings  = EXCLUDED.critical_findings,
        high_findings      = EXCLUDED.high_findings,
        medium_findings    = EXCLUDED.medium_findings,
        low_findings       = EXCLUDED.low_findings,
        highest_severity   = EXCLUDED.highest_severity,
        findings_json      = EXCLUDED.findings_json,
        error_message      = EXCLUDED.error_message,
        scanned_at         = now()
RETURNING *;

-- name: GetScanResult :one
SELECT sr.*
FROM scan_results sr
JOIN package_versions pv ON pv.id = sr.package_version_id
JOIN packages p ON p.id = pv.package_id
WHERE p.ecosystem = $1 AND p.name = $2 AND pv.version = $3;

-- name: ListRecentScans :many
SELECT
    p.ecosystem,
    p.name,
    pv.version,
    sr.highest_severity,
    sr.total_findings,
    sr.critical_findings,
    sr.high_findings,
    sr.scanned_at
FROM scan_results sr
JOIN package_versions pv ON pv.id = sr.package_version_id
JOIN packages p ON p.id = pv.package_id
WHERE sr.status = 'complete'
ORDER BY sr.scanned_at DESC
LIMIT $1;

-- name: DashboardStats :one
SELECT
    (SELECT COUNT(*)  FROM packages)                                       AS total_packages,
    (SELECT COUNT(*)  FROM package_versions)                               AS total_versions,
    (SELECT COALESCE(SUM(total_findings), 0) FROM scan_results)            AS total_findings,
    (SELECT COALESCE(SUM(critical_findings), 0) FROM scan_results)         AS critical_findings,
    (SELECT COALESCE(SUM(high_findings), 0)     FROM scan_results)         AS high_findings,
    (SELECT COUNT(*) FROM scan_results WHERE scanned_at >= now() - interval '24 hours') AS scanned_today;
