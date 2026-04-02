package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBRawListingRepository struct {
	db *sql.DB
}

func NewMariaDBRawListingRepository(db *sql.DB) *MariaDBRawListingRepository {
	return &MariaDBRawListingRepository{db: db}
}

func (r *MariaDBRawListingRepository) CreateBatch(ctx context.Context, listings []domain.RawListing) error {
	if len(listings) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin raw listings batch: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO raw_listings (
			id, scan_job_id, source, title, normalized_title, seller_name,
			price, original_price, is_promo, url, scraped_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare raw listings batch: %w", err)
	}
	defer stmt.Close()

	for _, listing := range listings {
		if _, err := stmt.ExecContext(
			ctx,
			listing.ID,
			listing.ScanJobID,
			listing.Source,
			listing.Title,
			listing.NormalizedTitle,
			listing.SellerName,
			listing.Price,
			nullableInt64(listing.OriginalPrice),
			listing.IsPromo,
			listing.URL,
			listing.ScrapedAt,
		); err != nil {
			return fmt.Errorf("insert raw listing: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit raw listings batch: %w", err)
	}

	return nil
}

func (r *MariaDBRawListingRepository) ListByScanJobID(ctx context.Context, scanJobID string) ([]domain.RawListing, error) {
	const query = `
		SELECT id, scan_job_id, source, title, normalized_title, seller_name,
			price, original_price, is_promo, url, scraped_at
		FROM raw_listings
		WHERE scan_job_id = ?
		ORDER BY price ASC, id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, scanJobID)
	if err != nil {
		return nil, fmt.Errorf("list raw listings by scan job id: %w", err)
	}
	defer rows.Close()

	var listings []domain.RawListing
	for rows.Next() {
		var (
			listing       domain.RawListing
			originalPrice sql.NullInt64
		)

		if err := rows.Scan(
			&listing.ID,
			&listing.ScanJobID,
			&listing.Source,
			&listing.Title,
			&listing.NormalizedTitle,
			&listing.SellerName,
			&listing.Price,
			&originalPrice,
			&listing.IsPromo,
			&listing.URL,
			&listing.ScrapedAt,
		); err != nil {
			return nil, fmt.Errorf("scan raw listing: %w", err)
		}

		listing.OriginalPrice = int64Pointer(originalPrice)
		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw listings: %w", err)
	}

	return listings, nil
}

var _ RawListingRepository = (*MariaDBRawListingRepository)(nil)
