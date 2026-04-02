package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBScanJobRepository struct {
	db *sql.DB
}

func NewMariaDBScanJobRepository(db *sql.DB) *MariaDBScanJobRepository {
	return &MariaDBScanJobRepository{db: db}
}

func (r *MariaDBScanJobRepository) Create(ctx context.Context, scanJob domain.ScanJob) error {
	const query = `
		INSERT INTO scan_jobs (
			id, tracked_keyword_id, started_at, finished_at, status,
			error_message, raw_count, grouped_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		scanJob.ID,
		scanJob.TrackedKeywordID,
		scanJob.StartedAt,
		nullableTime(scanJob.FinishedAt),
		string(scanJob.Status),
		nullableString(scanJob.ErrorMessage),
		scanJob.RawCount,
		scanJob.GroupedCount,
	)
	if err != nil {
		return fmt.Errorf("create scan job: %w", err)
	}

	return nil
}

func (r *MariaDBScanJobRepository) MarkSuccess(ctx context.Context, id string, rawCount, groupedCount int) error {
	const query = `
		UPDATE scan_jobs
		SET finished_at = UTC_TIMESTAMP(), status = ?, error_message = NULL,
			raw_count = ?, grouped_count = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, string(domain.ScanJobStatusSuccess), rawCount, groupedCount, id)
	if err != nil {
		return fmt.Errorf("mark scan job success: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark scan job success rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *MariaDBScanJobRepository) MarkFailed(ctx context.Context, id string, errorMessage string) error {
	const query = `
		UPDATE scan_jobs
		SET finished_at = UTC_TIMESTAMP(), status = ?, error_message = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, string(domain.ScanJobStatusFailed), errorMessage, id)
	if err != nil {
		return fmt.Errorf("mark scan job failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark scan job failed rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *MariaDBScanJobRepository) GetLatestByKeywordID(ctx context.Context, trackedKeywordID string) (*domain.ScanJob, error) {
	const query = `
		SELECT id, tracked_keyword_id, started_at, finished_at, status,
			error_message, raw_count, grouped_count
		FROM scan_jobs
		WHERE tracked_keyword_id = ?
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`

	var (
		scanJob      domain.ScanJob
		finishedAt   sql.NullTime
		errorMessage sql.NullString
		rawCount     sql.NullInt64
		groupedCount sql.NullInt64
		status       string
	)

	err := r.db.QueryRowContext(ctx, query, trackedKeywordID).Scan(
		&scanJob.ID,
		&scanJob.TrackedKeywordID,
		&scanJob.StartedAt,
		&finishedAt,
		&status,
		&errorMessage,
		&rawCount,
		&groupedCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}

		return nil, fmt.Errorf("get latest scan job by keyword id: %w", err)
	}

	scanJob.FinishedAt = timePointer(finishedAt)
	scanJob.ErrorMessage = stringPointer(errorMessage)
	if rawCount.Valid {
		scanJob.RawCount = int(rawCount.Int64)
	}
	if groupedCount.Valid {
		scanJob.GroupedCount = int(groupedCount.Int64)
	}
	scanJob.Status = domain.ScanJobStatus(status)

	return &scanJob, nil
}

var _ ScanJobRepository = (*MariaDBScanJobRepository)(nil)
