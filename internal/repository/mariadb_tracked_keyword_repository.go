package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBTrackedKeywordRepository struct {
	db *sql.DB
}

func NewMariaDBTrackedKeywordRepository(db *sql.DB) *MariaDBTrackedKeywordRepository {
	return &MariaDBTrackedKeywordRepository{db: db}
}

func (r *MariaDBTrackedKeywordRepository) Create(ctx context.Context, keyword domain.TrackedKeyword) error {
	const query = `
		INSERT INTO tracked_keywords (
			id, keyword, basic_filter, threshold_price, interval_minutes,
			telegram_enabled, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		keyword.ID,
		keyword.Keyword,
		nullableString(keyword.BasicFilter),
		nullableInt64(keyword.ThresholdPrice),
		keyword.IntervalMinutes,
		keyword.TelegramEnabled,
		string(keyword.Status),
		keyword.CreatedAt,
		keyword.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create tracked keyword: %w", err)
	}

	return nil
}

func (r *MariaDBTrackedKeywordRepository) Update(ctx context.Context, keyword domain.TrackedKeyword) error {
	const query = `
		UPDATE tracked_keywords
		SET keyword = ?, basic_filter = ?, threshold_price = ?, interval_minutes = ?,
			telegram_enabled = ?, status = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		keyword.Keyword,
		nullableString(keyword.BasicFilter),
		nullableInt64(keyword.ThresholdPrice),
		keyword.IntervalMinutes,
		keyword.TelegramEnabled,
		string(keyword.Status),
		keyword.UpdatedAt,
		keyword.ID,
	)
	if err != nil {
		return fmt.Errorf("update tracked keyword: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update tracked keyword rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *MariaDBTrackedKeywordRepository) GetByID(ctx context.Context, id string) (*domain.TrackedKeyword, error) {
	const query = `
		SELECT id, keyword, basic_filter, threshold_price, interval_minutes,
			telegram_enabled, status, created_at, updated_at
		FROM tracked_keywords
		WHERE id = ?
	`

	var (
		keyword        domain.TrackedKeyword
		basicFilter    sql.NullString
		thresholdPrice sql.NullInt64
		status         string
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&keyword.ID,
		&keyword.Keyword,
		&basicFilter,
		&thresholdPrice,
		&keyword.IntervalMinutes,
		&keyword.TelegramEnabled,
		&status,
		&keyword.CreatedAt,
		&keyword.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}

		return nil, fmt.Errorf("get tracked keyword by id: %w", err)
	}

	keyword.BasicFilter = stringPointer(basicFilter)
	keyword.ThresholdPrice = int64Pointer(thresholdPrice)
	keyword.Status = domain.TrackedKeywordStatus(status)

	return &keyword, nil
}

func (r *MariaDBTrackedKeywordRepository) ListActive(ctx context.Context) ([]domain.TrackedKeyword, error) {
	const query = `
		SELECT id, keyword, basic_filter, threshold_price, interval_minutes,
			telegram_enabled, status, created_at, updated_at
		FROM tracked_keywords
		WHERE status = ?
		ORDER BY updated_at DESC, id DESC
	`

	rows, err := r.db.QueryContext(ctx, query, string(domain.TrackedKeywordStatusActive))
	if err != nil {
		return nil, fmt.Errorf("list active tracked keywords: %w", err)
	}
	defer rows.Close()

	var keywords []domain.TrackedKeyword
	for rows.Next() {
		var (
			keyword        domain.TrackedKeyword
			basicFilter    sql.NullString
			thresholdPrice sql.NullInt64
			status         string
		)

		if err := rows.Scan(
			&keyword.ID,
			&keyword.Keyword,
			&basicFilter,
			&thresholdPrice,
			&keyword.IntervalMinutes,
			&keyword.TelegramEnabled,
			&status,
			&keyword.CreatedAt,
			&keyword.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active tracked keyword: %w", err)
		}

		keyword.BasicFilter = stringPointer(basicFilter)
		keyword.ThresholdPrice = int64Pointer(thresholdPrice)
		keyword.Status = domain.TrackedKeywordStatus(status)
		keywords = append(keywords, keyword)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active tracked keywords: %w", err)
	}

	return keywords, nil
}

var _ TrackedKeywordRepository = (*MariaDBTrackedKeywordRepository)(nil)
