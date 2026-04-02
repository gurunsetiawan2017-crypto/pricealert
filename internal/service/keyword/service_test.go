package keyword

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

func TestAddKeywordCreatesActiveTrackedKeyword(t *testing.T) {
	repo := &fakeTrackedKeywordRepo{}
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, repo, 5)

	err := service.AddKeyword(context.Background(), "  minyak goreng 2L  ")
	if err != nil {
		t.Fatalf("AddKeyword() error = %v", err)
	}
	if repo.created == nil {
		t.Fatalf("expected keyword to be created")
	}
	if repo.created.ID != "kw_1" {
		t.Fatalf("id = %q", repo.created.ID)
	}
	if repo.created.Keyword != "minyak goreng 2L" {
		t.Fatalf("keyword = %q", repo.created.Keyword)
	}
	if repo.created.Status != domain.TrackedKeywordStatusActive {
		t.Fatalf("status = %q", repo.created.Status)
	}
	if repo.created.IntervalMinutes != 5 {
		t.Fatalf("interval = %d", repo.created.IntervalMinutes)
	}
}

func TestPauseResumeArchiveUpdateStatus(t *testing.T) {
	repo := &fakeTrackedKeywordRepo{
		byID: map[string]*domain.TrackedKeyword{
			"kw_1": {ID: "kw_1", Keyword: "minyak goreng 2L", Status: domain.TrackedKeywordStatusActive},
		},
	}
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, repo, 5)

	if err := service.PauseKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("PauseKeyword() error = %v", err)
	}
	if repo.updated.Status != domain.TrackedKeywordStatusPaused {
		t.Fatalf("paused status = %q", repo.updated.Status)
	}

	if err := service.ResumeKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("ResumeKeyword() error = %v", err)
	}
	if repo.updated.Status != domain.TrackedKeywordStatusActive {
		t.Fatalf("resumed status = %q", repo.updated.Status)
	}

	if err := service.ArchiveKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("ArchiveKeyword() error = %v", err)
	}
	if repo.updated.Status != domain.TrackedKeywordStatusArchived {
		t.Fatalf("archived status = %q", repo.updated.Status)
	}
}

func TestAddKeywordRejectsEmptyValue(t *testing.T) {
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, &fakeTrackedKeywordRepo{}, 5)

	err := service.AddKeyword(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestUpdateStatusPropagatesGetByIDError(t *testing.T) {
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, &fakeTrackedKeywordRepo{err: errors.New("boom")}, 5)

	err := service.PauseKeyword(context.Background(), "kw_1")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestUpdateThresholdIntervalBasicFilterAndTelegram(t *testing.T) {
	threshold := int64(25000)
	filter := "2L"
	repo := &fakeTrackedKeywordRepo{
		byID: map[string]*domain.TrackedKeyword{
			"kw_1": {ID: "kw_1", Keyword: "minyak goreng", Status: domain.TrackedKeywordStatusActive},
		},
	}
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, repo, 5)

	if err := service.UpdateThreshold(context.Background(), "kw_1", &threshold); err != nil {
		t.Fatalf("UpdateThreshold() error = %v", err)
	}
	if repo.updated.ThresholdPrice == nil || *repo.updated.ThresholdPrice != 25000 {
		t.Fatalf("threshold = %v", repo.updated.ThresholdPrice)
	}

	if err := service.UpdateInterval(context.Background(), "kw_1", 10); err != nil {
		t.Fatalf("UpdateInterval() error = %v", err)
	}
	if repo.updated.IntervalMinutes != 10 {
		t.Fatalf("interval = %d", repo.updated.IntervalMinutes)
	}

	if err := service.UpdateBasicFilter(context.Background(), "kw_1", &filter); err != nil {
		t.Fatalf("UpdateBasicFilter() error = %v", err)
	}
	if repo.updated.BasicFilter == nil || *repo.updated.BasicFilter != "2L" {
		t.Fatalf("basic filter = %v", repo.updated.BasicFilter)
	}

	if err := service.SetTelegramEnabled(context.Background(), "kw_1", true); err != nil {
		t.Fatalf("SetTelegramEnabled() error = %v", err)
	}
	if !repo.updated.TelegramEnabled {
		t.Fatalf("telegram enabled = false")
	}
}

func TestUpdateValidationRejectsInvalidValues(t *testing.T) {
	repo := &fakeTrackedKeywordRepo{
		byID: map[string]*domain.TrackedKeyword{
			"kw_1": {ID: "kw_1", Keyword: "minyak goreng", Status: domain.TrackedKeywordStatusActive},
		},
	}
	service := NewService(fakeIDGen{id: "kw_1"}, fakeClock{now: fixedTime()}, repo, 5)

	invalidThreshold := int64(0)
	if err := service.UpdateThreshold(context.Background(), "kw_1", &invalidThreshold); err == nil {
		t.Fatalf("expected threshold validation error")
	}
	if err := service.UpdateInterval(context.Background(), "kw_1", 0); err == nil {
		t.Fatalf("expected interval validation error")
	}
}

type fakeTrackedKeywordRepo struct {
	created *domain.TrackedKeyword
	updated domain.TrackedKeyword
	byID    map[string]*domain.TrackedKeyword
	err     error
}

func (f *fakeTrackedKeywordRepo) Create(_ context.Context, keyword domain.TrackedKeyword) error {
	f.created = &keyword
	return nil
}
func (f *fakeTrackedKeywordRepo) Update(_ context.Context, keyword domain.TrackedKeyword) error {
	f.updated = keyword
	if existing, ok := f.byID[keyword.ID]; ok {
		*existing = keyword
	}
	return nil
}
func (f *fakeTrackedKeywordRepo) GetByID(_ context.Context, id string) (*domain.TrackedKeyword, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[id], nil
}
func (f *fakeTrackedKeywordRepo) ListActive(context.Context) ([]domain.TrackedKeyword, error) {
	return nil, nil
}
func (f *fakeTrackedKeywordRepo) ListVisible(context.Context) ([]domain.TrackedKeyword, error) {
	return nil, nil
}

type fakeIDGen struct{ id string }

func (f fakeIDGen) Next() string { return f.id }

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

func fixedTime() time.Time {
	return time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
}
