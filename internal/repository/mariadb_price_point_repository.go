package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBPricePointRepository struct {
	db *sql.DB
}

func NewMariaDBPricePointRepository(db *sql.DB) *MariaDBPricePointRepository {
	return &MariaDBPricePointRepository{db: db}
}

func (r *MariaDBPricePointRepository) Create(ctx context.Context, point domain.PricePoint) error {
	const query = `
		INSERT INTO price_points (
			id, tracked_keyword_id, scan_job_id, min_price, avg_price, max_price, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		point.ID,
		point.TrackedKeywordID,
		point.ScanJobID,
		nullableInt64(point.MinPrice),
		nullableInt64(point.AvgPrice),
		nullableInt64(point.MaxPrice),
		point.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("create price point: %w", err)
	}

	return nil
}

func (r *MariaDBPricePointRepository) ListRecentByKeywordID(ctx context.Context, trackedKeywordID string, limit int) ([]domain.PricePoint, error) {
	if limit <= 0 {
		return []domain.PricePoint{}, nil
	}

	const query = `
		SELECT id, tracked_keyword_id, scan_job_id, min_price, avg_price, max_price, recorded_at
		FROM price_points
		WHERE tracked_keyword_id = ?
		ORDER BY recorded_at DESC, id DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, trackedKeywordID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent price points by keyword id: %w", err)
	}
	defer rows.Close()

	var points []domain.PricePoint
	for rows.Next() {
		var (
			point    domain.PricePoint
			minPrice sql.NullInt64
			avgPrice sql.NullInt64
			maxPrice sql.NullInt64
		)

		if err := rows.Scan(
			&point.ID,
			&point.TrackedKeywordID,
			&point.ScanJobID,
			&minPrice,
			&avgPrice,
			&maxPrice,
			&point.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan price point: %w", err)
		}

		point.MinPrice = int64Pointer(minPrice)
		point.AvgPrice = int64Pointer(avgPrice)
		point.MaxPrice = int64Pointer(maxPrice)
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate price points: %w", err)
	}

	return points, nil
}

func (r *MariaDBPricePointRepository) PruneOlderThanRecordedAt(ctx context.Context, cutoff time.Time) (int, error) {
	const query = `
		DELETE FROM price_points
		WHERE recorded_at < ?
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune price points older than cutoff: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune price points rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

var _ PricePointRepository = (*MariaDBPricePointRepository)(nil)
