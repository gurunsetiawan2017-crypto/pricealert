package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

type MariaDBAlertEventRepository struct {
	db *sql.DB
}

func NewMariaDBAlertEventRepository(db *sql.DB) *MariaDBAlertEventRepository {
	return &MariaDBAlertEventRepository{db: db}
}

func (r *MariaDBAlertEventRepository) Create(ctx context.Context, event domain.AlertEvent) error {
	const query = `
		INSERT INTO alert_events (
			id, tracked_keyword_id, scan_job_id, level, event_type,
			message, payload_json, sent_to_telegram, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		event.ID,
		event.TrackedKeywordID,
		nullableString(event.ScanJobID),
		string(event.Level),
		string(event.EventType),
		event.Message,
		nullableString(event.PayloadJSON),
		event.SentToTelegram,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert event: %w", err)
	}

	return nil
}

func (r *MariaDBAlertEventRepository) MarkSentToTelegram(ctx context.Context, eventID string) error {
	const query = `
		UPDATE alert_events
		SET sent_to_telegram = TRUE
		WHERE id = ?
	`

	if _, err := r.db.ExecContext(ctx, query, eventID); err != nil {
		return fmt.Errorf("mark alert event sent to telegram: %w", err)
	}

	return nil
}

func (r *MariaDBAlertEventRepository) ListRecentByKeywordID(ctx context.Context, trackedKeywordID string, limit int) ([]domain.AlertEvent, error) {
	if limit <= 0 {
		return []domain.AlertEvent{}, nil
	}

	const query = `
		SELECT id, tracked_keyword_id, scan_job_id, level, event_type,
			message, payload_json, sent_to_telegram, created_at
		FROM alert_events
		WHERE tracked_keyword_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, trackedKeywordID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent alert events by keyword id: %w", err)
	}
	defer rows.Close()

	var events []domain.AlertEvent
	for rows.Next() {
		var (
			event       domain.AlertEvent
			scanJobID   sql.NullString
			payloadJSON sql.NullString
			level       string
			eventType   string
		)

		if err := rows.Scan(
			&event.ID,
			&event.TrackedKeywordID,
			&scanJobID,
			&level,
			&eventType,
			&event.Message,
			&payloadJSON,
			&event.SentToTelegram,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
		}

		event.ScanJobID = stringPointer(scanJobID)
		event.PayloadJSON = stringPointer(payloadJSON)
		event.Level = domain.AlertLevel(level)
		event.EventType = domain.AlertEventType(eventType)
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert events: %w", err)
	}

	return events, nil
}

func (r *MariaDBAlertEventRepository) PruneOlderThanCreatedAt(ctx context.Context, cutoff time.Time) (int, error) {
	const query = `
		DELETE FROM alert_events
		WHERE created_at < ?
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune alert events older than cutoff: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune alert events rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

var _ AlertEventRepository = (*MariaDBAlertEventRepository)(nil)
