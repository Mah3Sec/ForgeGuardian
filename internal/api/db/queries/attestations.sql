-- name: InsertAttestation :one
INSERT INTO attestations (
    package_version_id, sha256, signature, public_key,
    rekor_log_id, rekor_index, rekor_url, attestation_json, signed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
RETURNING *;

-- name: GetAttestationBySHA256 :one
SELECT att.*, p.ecosystem, p.name, pv.version
FROM attestations att
JOIN package_versions pv ON pv.id = att.package_version_id
JOIN packages p ON p.id = pv.package_id
WHERE att.sha256 = $1
ORDER BY att.signed_at DESC
LIMIT 1;
