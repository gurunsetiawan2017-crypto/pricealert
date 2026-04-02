package repository

import (
	"database/sql"
	"time"
)

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{
		String: *value,
		Valid:  true,
	}
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}

	return sql.NullInt64{
		Int64: *value,
		Valid: true,
	}
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{
		Time:  *value,
		Valid: true,
	}
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	v := value.String
	return &v
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}

	v := value.Int64
	return &v
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	v := value.Time
	return &v
}
