package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBGroupedListingRepository struct {
	db *sql.DB
}

func NewMariaDBGroupedListingRepository(db *sql.DB) *MariaDBGroupedListingRepository {
	return &MariaDBGroupedListingRepository{db: db}
}

func (r *MariaDBGroupedListingRepository) CreateBatch(ctx context.Context, listings []domain.GroupedListing) error {
	if len(listings) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin grouped listings batch: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO grouped_listings (
			id, scan_job_id, group_key, representative_title, representative_seller,
			best_price, original_price, is_promo, listing_count, sample_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare grouped listings batch: %w", err)
	}
	defer stmt.Close()

	for _, listing := range listings {
		if _, err := stmt.ExecContext(
			ctx,
			listing.ID,
			listing.ScanJobID,
			listing.GroupKey,
			listing.RepresentativeTitle,
			listing.RepresentativeSeller,
			listing.BestPrice,
			nullableInt64(listing.OriginalPrice),
			listing.IsPromo,
			listing.ListingCount,
			listing.SampleURL,
		); err != nil {
			return fmt.Errorf("insert grouped listing: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit grouped listings batch: %w", err)
	}

	return nil
}

func (r *MariaDBGroupedListingRepository) ListByScanJobID(ctx context.Context, scanJobID string) ([]domain.GroupedListing, error) {
	const query = `
		SELECT id, scan_job_id, group_key, representative_title, representative_seller,
			best_price, original_price, is_promo, listing_count, sample_url
		FROM grouped_listings
		WHERE scan_job_id = ?
		ORDER BY best_price ASC, id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, scanJobID)
	if err != nil {
		return nil, fmt.Errorf("list grouped listings by scan job id: %w", err)
	}
	defer rows.Close()

	var listings []domain.GroupedListing
	for rows.Next() {
		var (
			listing       domain.GroupedListing
			originalPrice sql.NullInt64
		)

		if err := rows.Scan(
			&listing.ID,
			&listing.ScanJobID,
			&listing.GroupKey,
			&listing.RepresentativeTitle,
			&listing.RepresentativeSeller,
			&listing.BestPrice,
			&originalPrice,
			&listing.IsPromo,
			&listing.ListingCount,
			&listing.SampleURL,
		); err != nil {
			return nil, fmt.Errorf("scan grouped listing: %w", err)
		}

		listing.OriginalPrice = int64Pointer(originalPrice)
		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grouped listings: %w", err)
	}

	return listings, nil
}

var _ GroupedListingRepository = (*MariaDBGroupedListingRepository)(nil)
