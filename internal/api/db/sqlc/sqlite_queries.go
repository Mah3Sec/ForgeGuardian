package dbsqlc

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// SQLiteQueries implements Querier against an embedded SQLite database.
type SQLiteQueries struct {
	db *sql.DB
}

func NewSQLite(db *sql.DB) *SQLiteQueries { return &SQLiteQueries{db: db} }

var _ Querier = (*SQLiteQueries)(nil)

func toPgTimestamptz(s string) pgtype.Timestamptz {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	return pgtype.Timestamptz{}
}

func pgTimestamptzToStr(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return ts.Time.UTC().Format(time.RFC3339)
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

// ─── packages ────────────────────────────────────────────────────────────────

func (q *SQLiteQueries) UpsertPackage(ctx context.Context, ecosystem, name string) (Package, error) {
	now := nowStr()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO packages (ecosystem, name, created_at, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (ecosystem, name) DO UPDATE SET updated_at = ?`,
		ecosystem, name, now, now, now)
	if err != nil {
		return Package{}, err
	}
	var p Package
	var createdAt, updatedAt string
	err = q.db.QueryRowContext(ctx,
		`SELECT id, ecosystem, name, created_at, updated_at FROM packages WHERE ecosystem = ? AND name = ?`,
		ecosystem, name).Scan(&p.ID, &p.Ecosystem, &p.Name, &createdAt, &updatedAt)
	if err != nil {
		return Package{}, err
	}
	p.CreatedAt = toPgTimestamptz(createdAt)
	p.UpdatedAt = toPgTimestamptz(updatedAt)
	return p, nil
}

func (q *SQLiteQueries) GetPackage(ctx context.Context, ecosystem, name string) (PackageWithVersionCount, error) {
	var r PackageWithVersionCount
	var createdAt, updatedAt string
	err := q.db.QueryRowContext(ctx,
		`SELECT p.id, p.ecosystem, p.name, p.created_at, p.updated_at, COUNT(pv.id) AS version_count
		 FROM packages p LEFT JOIN package_versions pv ON pv.package_id = p.id
		 WHERE p.ecosystem = ? AND p.name = ? GROUP BY p.id`,
		ecosystem, name).Scan(&r.ID, &r.Ecosystem, &r.Name, &createdAt, &updatedAt, &r.VersionCount)
	if err != nil {
		return PackageWithVersionCount{}, err
	}
	r.CreatedAt = toPgTimestamptz(createdAt)
	r.UpdatedAt = toPgTimestamptz(updatedAt)
	return r, nil
}

func (q *SQLiteQueries) ListPackages(ctx context.Context, ecosystem string, limit, offset int32) ([]PackageWithVersionCount, error) {
	query := `SELECT p.id, p.ecosystem, p.name, p.created_at, p.updated_at, COUNT(pv.id) AS version_count
	          FROM packages p LEFT JOIN package_versions pv ON pv.package_id = p.id`
	var args []any
	if ecosystem != "" {
		query += ` WHERE p.ecosystem = ?`
		args = append(args, ecosystem)
	}
	query += ` GROUP BY p.id ORDER BY p.updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PackageWithVersionCount
	for rows.Next() {
		var r PackageWithVersionCount
		var createdAt, updatedAt string
		if err := rows.Scan(&r.ID, &r.Ecosystem, &r.Name, &createdAt, &updatedAt, &r.VersionCount); err != nil {
			return nil, err
		}
		r.CreatedAt = toPgTimestamptz(createdAt)
		r.UpdatedAt = toPgTimestamptz(updatedAt)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (q *SQLiteQueries) CountPackages(ctx context.Context, ecosystem string) (int64, error) {
	query := `SELECT COUNT(*) FROM packages`
	var args []any
	if ecosystem != "" {
		query += ` WHERE ecosystem = ?`
		args = append(args, ecosystem)
	}
	var count int64
	err := q.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// ─── package versions ────────────────────────────────────────────────────────

func (q *SQLiteQueries) UpsertPackageVersion(ctx context.Context, arg UpsertPackageVersionParams) (PackageVersion, error) {
	now := nowStr()
	pubAt := pgTimestamptzToStr(arg.PublishedAt)

	_, err := q.db.ExecContext(ctx,
		`INSERT INTO package_versions (package_id, version, source_url, checksum, published_at, scan_status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'pending', ?, ?)
		 ON CONFLICT (package_id, version) DO UPDATE SET source_url = excluded.source_url, checksum = excluded.checksum, updated_at = ?`,
		arg.PackageID, arg.Version, arg.SourceURL, arg.Checksum, pubAt, now, now, now)
	if err != nil {
		return PackageVersion{}, err
	}

	var pv PackageVersion
	var publishedAt, createdAt string
	err = q.db.QueryRowContext(ctx,
		`SELECT id, package_id, version, source_url, checksum, published_at, created_at
		 FROM package_versions WHERE package_id = ? AND version = ?`,
		arg.PackageID, arg.Version).Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.SourceURL, &pv.Checksum, &publishedAt, &createdAt)
	if err != nil {
		return PackageVersion{}, err
	}
	pv.PublishedAt = toPgTimestamptz(publishedAt)
	pv.CreatedAt = toPgTimestamptz(createdAt)
	return pv, nil
}

func (q *SQLiteQueries) ListVersions(ctx context.Context, ecosystem, name string) ([]PackageVersion, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT pv.id, pv.package_id, pv.version, pv.source_url, pv.checksum, pv.published_at, pv.created_at
		 FROM package_versions pv
		 JOIN packages p ON p.id = pv.package_id
		 WHERE p.ecosystem = ? AND p.name = ?
		 ORDER BY pv.created_at DESC`, ecosystem, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PackageVersion
	for rows.Next() {
		var pv PackageVersion
		var publishedAt, createdAt string
		if err := rows.Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.SourceURL, &pv.Checksum, &publishedAt, &createdAt); err != nil {
			return nil, err
		}
		pv.PublishedAt = toPgTimestamptz(publishedAt)
		pv.CreatedAt = toPgTimestamptz(createdAt)
		results = append(results, pv)
	}
	return results, rows.Err()
}

func (q *SQLiteQueries) GetPackageVersion(ctx context.Context, ecosystem, name, version string) (PackageVersion, error) {
	var pv PackageVersion
	var publishedAt, createdAt string
	err := q.db.QueryRowContext(ctx,
		`SELECT pv.id, pv.package_id, pv.version, pv.source_url, pv.checksum, pv.published_at, pv.created_at
		 FROM package_versions pv
		 JOIN packages p ON p.id = pv.package_id
		 WHERE p.ecosystem = ? AND p.name = ? AND pv.version = ?`,
		ecosystem, name, version).Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.SourceURL, &pv.Checksum, &publishedAt, &createdAt)
	if err != nil {
		return PackageVersion{}, err
	}
	pv.PublishedAt = toPgTimestamptz(publishedAt)
	pv.CreatedAt = toPgTimestamptz(createdAt)
	return pv, nil
}

// ─── scan results ────────────────────────────────────────────────────────────

func (q *SQLiteQueries) UpsertScanResult(ctx context.Context, arg UpsertScanResultParams) (ScanResult, error) {
	now := nowStr()
	_, err := q.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO scan_results
		 (package_version_id, sha256, status, total_findings, critical_findings, high_findings, medium_findings, low_findings, highest_severity, findings_json, error_message, scanned_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.PackageVersionID, arg.Sha256, arg.Status,
		arg.TotalFindings, arg.CriticalFindings, arg.HighFindings, arg.MediumFindings, arg.LowFindings,
		arg.HighestSeverity, string(arg.FindingsJson), arg.ErrorMessage, now)
	if err != nil {
		return ScanResult{}, err
	}

	var sr ScanResult
	var scannedAt, findingsStr string
	err = q.db.QueryRowContext(ctx,
		`SELECT id, package_version_id, sha256, status, total_findings, critical_findings, high_findings, medium_findings, low_findings, highest_severity, findings_json, error_message, scanned_at
		 FROM scan_results WHERE package_version_id = ? ORDER BY id DESC LIMIT 1`, arg.PackageVersionID).Scan(
		&sr.ID, &sr.PackageVersionID, &sr.Sha256, &sr.Status,
		&sr.TotalFindings, &sr.CriticalFindings, &sr.HighFindings, &sr.MediumFindings, &sr.LowFindings,
		&sr.HighestSeverity, &findingsStr, &sr.ErrorMessage, &scannedAt)
	if err != nil {
		return ScanResult{}, err
	}
	sr.FindingsJson = []byte(findingsStr)
	sr.ScannedAt = toPgTimestamptz(scannedAt)
	return sr, nil
}

func (q *SQLiteQueries) GetScanResult(ctx context.Context, ecosystem, name, version string) (ScanResult, error) {
	var sr ScanResult
	var scannedAt, findingsStr string
	err := q.db.QueryRowContext(ctx,
		`SELECT sr.id, sr.package_version_id, sr.sha256, sr.status,
		        sr.total_findings, sr.critical_findings, sr.high_findings, sr.medium_findings, sr.low_findings,
		        sr.highest_severity, sr.findings_json, sr.error_message, sr.scanned_at
		 FROM scan_results sr
		 JOIN package_versions pv ON pv.id = sr.package_version_id
		 JOIN packages p ON p.id = pv.package_id
		 WHERE p.ecosystem = ? AND p.name = ? AND pv.version = ?
		 ORDER BY sr.id DESC LIMIT 1`,
		ecosystem, name, version).Scan(
		&sr.ID, &sr.PackageVersionID, &sr.Sha256, &sr.Status,
		&sr.TotalFindings, &sr.CriticalFindings, &sr.HighFindings, &sr.MediumFindings, &sr.LowFindings,
		&sr.HighestSeverity, &findingsStr, &sr.ErrorMessage, &scannedAt)
	if err != nil {
		return ScanResult{}, err
	}
	sr.FindingsJson = []byte(findingsStr)
	sr.ScannedAt = toPgTimestamptz(scannedAt)
	return sr, nil
}

func (q *SQLiteQueries) ListRecentScans(ctx context.Context, limit int32) ([]RecentScanRow, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT p.ecosystem, p.name, pv.version, sr.highest_severity,
		        sr.total_findings, sr.critical_findings, sr.high_findings, sr.medium_findings, sr.low_findings, sr.scanned_at
		 FROM scan_results sr
		 JOIN package_versions pv ON pv.id = sr.package_version_id
		 JOIN packages p ON p.id = pv.package_id
		 WHERE sr.status = 'complete'
		 ORDER BY sr.scanned_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RecentScanRow
	for rows.Next() {
		var r RecentScanRow
		var scannedAt string
		if err := rows.Scan(&r.Ecosystem, &r.Name, &r.Version, &r.HighestSeverity,
			&r.TotalFindings, &r.CriticalFindings, &r.HighFindings, &r.MediumFindings, &r.LowFindings, &scannedAt); err != nil {
			return nil, err
		}
		r.ScannedAt = toPgTimestamptz(scannedAt)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (q *SQLiteQueries) DashboardStats(ctx context.Context) (DashboardStatsRow, error) {
	var s DashboardStatsRow
	err := q.db.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM packages) AS total_packages,
			(SELECT COUNT(*) FROM package_versions) AS total_versions,
			COALESCE((SELECT SUM(total_findings) FROM scan_results), 0) AS total_findings,
			COALESCE((SELECT SUM(critical_findings) FROM scan_results), 0) AS critical_findings,
			COALESCE((SELECT SUM(high_findings) FROM scan_results), 0) AS high_findings,
			COALESCE((SELECT SUM(medium_findings) FROM scan_results), 0) AS medium_findings,
			COALESCE((SELECT SUM(low_findings) FROM scan_results), 0) AS low_findings,
			(SELECT COUNT(*) FROM scan_results WHERE date(scanned_at) = date('now')) AS scanned_today`).Scan(
		&s.TotalPackages, &s.TotalVersions, &s.TotalFindings,
		&s.CriticalFindings, &s.HighFindings, &s.MediumFindings, &s.LowFindings, &s.ScannedToday)
	return s, err
}

func (q *SQLiteQueries) DashboardTimeline(ctx context.Context, days int32) ([]DashboardTimelineRow, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT date(scanned_at) AS day,
		        SUM(critical_findings) AS critical,
		        SUM(high_findings) AS high,
		        SUM(medium_findings) AS medium,
		        SUM(low_findings) AS low
		 FROM scan_results
		 WHERE scanned_at >= datetime('now', ? || ' days')
		 GROUP BY date(scanned_at)
		 ORDER BY day`,
		-days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DashboardTimelineRow
	for rows.Next() {
		var r DashboardTimelineRow
		var day string
		if err := rows.Scan(&day, &r.Critical, &r.High, &r.Medium, &r.Low); err != nil {
			return nil, err
		}
		r.Day = toPgTimestamptz(day + "T00:00:00Z")
		results = append(results, r)
	}
	return results, rows.Err()
}

// ─── attestations ────────────────────────────────────────────────────────────

func (q *SQLiteQueries) InsertAttestation(ctx context.Context, arg InsertAttestationParams) (Attestation, error) {
	now := nowStr()
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO attestations (package_version_id, sha256, signature, public_key, rekor_log_id, rekor_index, rekor_url, attestation_json, signed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.PackageVersionID, arg.Sha256, arg.Signature, arg.PublicKey,
		arg.RekorLogID, arg.RekorIndex, arg.RekorURL, string(arg.AttestationJson), now)
	if err != nil {
		return Attestation{}, err
	}
	id, _ := res.LastInsertId()
	return Attestation{
		ID:               id,
		PackageVersionID: arg.PackageVersionID,
		Sha256:           arg.Sha256,
		Signature:        arg.Signature,
		PublicKey:        arg.PublicKey,
		RekorLogID:       arg.RekorLogID,
		RekorIndex:       arg.RekorIndex,
		RekorURL:         arg.RekorURL,
		AttestationJson:  arg.AttestationJson,
		SignedAt:         toPgTimestamptz(now),
	}, nil
}

func (q *SQLiteQueries) GetAttestationBySHA256(ctx context.Context, sha256 string) (AttestationWithPackage, error) {
	var a AttestationWithPackage
	var signedAt, attestationStr string
	err := q.db.QueryRowContext(ctx,
		`SELECT a.id, a.package_version_id, a.sha256, a.signature, a.public_key,
		        a.rekor_log_id, a.rekor_index, a.rekor_url, a.attestation_json, a.signed_at,
		        p.ecosystem, p.name, pv.version
		 FROM attestations a
		 JOIN package_versions pv ON pv.id = a.package_version_id
		 JOIN packages p ON p.id = pv.package_id
		 WHERE a.sha256 = ?
		 ORDER BY a.signed_at DESC LIMIT 1`, sha256).Scan(
		&a.ID, &a.PackageVersionID, &a.Sha256, &a.Signature, &a.PublicKey,
		&a.RekorLogID, &a.RekorIndex, &a.RekorURL, &attestationStr, &signedAt,
		&a.Ecosystem, &a.PkgName, &a.Version)
	if err != nil {
		return AttestationWithPackage{}, err
	}
	a.AttestationJson = []byte(attestationStr)
	a.SignedAt = toPgTimestamptz(signedAt)
	return a, nil
}

// ─── allowlist ───────────────────────────────────────────────────────────────

func (q *SQLiteQueries) ListAllowlist(ctx context.Context) ([]AllowlistEntry, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id, ecosystem, package, reason, added_by, created_at FROM allowlist ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []AllowlistEntry
	for rows.Next() {
		var e AllowlistEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Ecosystem, &e.Package, &e.Reason, &e.AddedBy, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = toPgTimestamptz(createdAt)
		results = append(results, e)
	}
	return results, rows.Err()
}

func (q *SQLiteQueries) UpsertAllowlist(ctx context.Context, ecosystem, pkg, reason, addedBy string) (AllowlistEntry, error) {
	now := nowStr()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO allowlist (ecosystem, package, reason, added_by, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (ecosystem, package) DO UPDATE SET reason = excluded.reason, added_by = excluded.added_by`,
		ecosystem, pkg, reason, addedBy, now)
	if err != nil {
		return AllowlistEntry{}, err
	}
	var e AllowlistEntry
	var createdAt string
	err = q.db.QueryRowContext(ctx,
		`SELECT id, ecosystem, package, reason, added_by, created_at FROM allowlist WHERE ecosystem = ? AND package = ?`,
		ecosystem, pkg).Scan(&e.ID, &e.Ecosystem, &e.Package, &e.Reason, &e.AddedBy, &createdAt)
	if err != nil {
		return AllowlistEntry{}, err
	}
	e.CreatedAt = toPgTimestamptz(createdAt)
	return e, nil
}

func (q *SQLiteQueries) DeleteAllowlist(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM allowlist WHERE id = ?`, id)
	return err
}

func (q *SQLiteQueries) IsAllowlisted(ctx context.Context, pkg, ecosystem string) (bool, error) {
	var n int64
	err := q.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM allowlist WHERE package = ? AND (ecosystem = '' OR ecosystem = ?)`,
		pkg, ecosystem).Scan(&n)
	return n > 0, err
}

// ─── alerts ──────────────────────────────────────────────────────────────────

func (q *SQLiteQueries) InsertAlert(ctx context.Context, typ, message, severity string, pkgName, eco, ver *string, meta []byte) (Alert, error) {
	now := nowStr()
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO alerts (type, message, severity, package_name, ecosystem, version, metadata, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		typ, message, severity, pkgName, eco, ver, string(meta), now)
	if err != nil {
		return Alert{}, err
	}
	id, _ := res.LastInsertId()
	return Alert{
		ID:          id,
		Type:        typ,
		Message:     message,
		Severity:    severity,
		PackageName: pkgName,
		Ecosystem:   eco,
		Version:     ver,
		Metadata:    meta,
		Dismissed:   false,
		OccurredAt:  toPgTimestamptz(now),
	}, nil
}

func (q *SQLiteQueries) ListAlerts(ctx context.Context, dismissed *bool, severity string, limit, offset int32) ([]Alert, error) {
	query := `SELECT id, type, message, severity, package_name, ecosystem, version, metadata, dismissed, occurred_at FROM alerts WHERE 1=1`
	var args []any
	if dismissed != nil {
		query += ` AND dismissed = ?`
		if *dismissed {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if severity != "" {
		query += ` AND severity = ?`
		args = append(args, severity)
	}
	query += ` ORDER BY occurred_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Alert
	for rows.Next() {
		var a Alert
		var occurredAt string
		var metaStr sql.NullString
		var dismissedInt int
		if err := rows.Scan(&a.ID, &a.Type, &a.Message, &a.Severity, &a.PackageName, &a.Ecosystem, &a.Version, &metaStr, &dismissedInt, &occurredAt); err != nil {
			return nil, err
		}
		a.Dismissed = dismissedInt != 0
		if metaStr.Valid {
			a.Metadata = []byte(metaStr.String)
		}
		a.OccurredAt = toPgTimestamptz(occurredAt)
		results = append(results, a)
	}
	return results, rows.Err()
}

func (q *SQLiteQueries) DismissAlert(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `UPDATE alerts SET dismissed = 1 WHERE id = ?`, id)
	return err
}

func (q *SQLiteQueries) CountAlerts(ctx context.Context, dismissed *bool, severity string) (int64, error) {
	query := `SELECT COUNT(*) FROM alerts WHERE 1=1`
	var args []any
	if dismissed != nil {
		query += ` AND dismissed = ?`
		if *dismissed {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if severity != "" {
		query += ` AND severity = ?`
		args = append(args, severity)
	}
	var n int64
	err := q.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}
