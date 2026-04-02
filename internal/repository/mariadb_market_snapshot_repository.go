package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBMarketSnapshotRepository struct {
	db *sql.DB
}

func NewMariaDBMarketSnapshotRepository(db *sql.DB) *MariaDBMarketSnapshotRepository {
	return &MariaDBMarketSnapshotRepository{db: db}
}

func (r *MariaDBMarketSnapshotRepository) Create(ctx context.Context, snapshot domain.MarketSnapshot) error {
	const query = `
		INSERT INTO market_snapshots (
			id, tracked_keyword_id, scan_job_id, min_price, avg_price, max_price,
			raw_count, grouped_count, signal, snapshot_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		snapshot.ID,
		snapshot.TrackedKeywordID,
		snapshot.ScanJobID,
		nullableInt64(snapshot.MinPrice),
		nullableInt64(snapshot.AvgPrice),
		nullableInt64(snapshot.MaxPrice),
		snapshot.RawCount,
		snapshot.GroupedCount,
		string(snapshot.Signal),
		snapshot.SnapshotAt,
	)
	if err != nil {
		return fmt.Errorf("create market snapshot: %w", err)
	}

	return nil
}

func (r *MariaDBMarketSnapshotRepository) GetLatestByKeywordID(ctx context.Context, trackedKeywordID string) (*domain.MarketSnapshot, error) {
	const query = `
		SELECT id, tracked_keyword_id, scan_job_id, min_price, avg_price, max_price,
			raw_count, grouped_count, signal, snapshot_at
		FROM market_snapshots
		WHERE tracked_keyword_id = ?
		ORDER BY snapshot_at DESC, id DESC
		LIMIT 1
	`

	var (
		snapshot domain.MarketSnapshot
		minPrice sql.NullInt64
		avgPrice sql.NullInt64
		maxPrice sql.NullInt64
		signal   string
	)

	err := r.db.QueryRowContext(ctx, query, trackedKeywordID).Scan(
		&snapshot.ID,
		&snapshot.TrackedKeywordID,
		&snapshot.ScanJobID,
		&minPrice,
		&avgPrice,
		&maxPrice,
		&snapshot.RawCount,
		&snapshot.GroupedCount,
		&signal,
		&snapshot.SnapshotAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}

		return nil, fmt.Errorf("get latest market snapshot by keyword id: %w", err)
	}

	snapshot.MinPrice = int64Pointer(minPrice)
	snapshot.AvgPrice = int64Pointer(avgPrice)
	snapshot.MaxPrice = int64Pointer(maxPrice)
	snapshot.Signal = domain.MarketSignal(signal)

	return &snapshot, nil
}

var _ MarketSnapshotRepository = (*MariaDBMarketSnapshotRepository)(nil)
