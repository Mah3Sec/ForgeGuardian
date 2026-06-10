-- name: UpsertPackage :one
INSERT INTO packages (ecosystem, name, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (ecosystem, name) DO UPDATE
    SET updated_at = now()
RETURNING *;

-- name: GetPackage :one
SELECT p.*, COUNT(pv.id) AS version_count
FROM packages p
LEFT JOIN package_versions pv ON pv.package_id = p.id
WHERE p.ecosystem = $1 AND p.name = $2
GROUP BY p.id;

-- name: ListPackages :many
SELECT p.*, COUNT(pv.id) AS version_count
FROM packages p
LEFT JOIN package_versions pv ON pv.package_id = p.id
WHERE ($1::text = '' OR p.ecosystem = $1)
GROUP BY p.id
ORDER BY p.updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPackages :one
SELECT COUNT(*) FROM packages
WHERE ($1::text = '' OR ecosystem = $1);

-- name: UpsertPackageVersion :one
INSERT INTO package_versions (package_id, version, source_url, checksum, published_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (package_id, version) DO UPDATE
    SET source_url = EXCLUDED.source_url,
        checksum   = EXCLUDED.checksum
RETURNING *;

-- name: ListVersions :many
SELECT pv.*
FROM package_versions pv
JOIN packages p ON p.id = pv.package_id
WHERE p.ecosystem = $1 AND p.name = $2
ORDER BY pv.created_at DESC;

-- name: GetPackageVersion :one
SELECT pv.*
FROM package_versions pv
JOIN packages p ON p.id = pv.package_id
WHERE p.ecosystem = $1 AND p.name = $2 AND pv.version = $3;
