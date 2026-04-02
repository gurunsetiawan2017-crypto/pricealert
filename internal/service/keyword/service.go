package keyword

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/repository"
)

type IDGenerator interface {
	Next() string
}

type Clock interface {
	Now() time.Time
}

type Service struct {
	idGenerator      IDGenerator
	clock            Clock
	trackedKeywords  repository.TrackedKeywordRepository
	defaultIntervalM int
}

func NewService(
	idGenerator IDGenerator,
	clock Clock,
	trackedKeywords repository.TrackedKeywordRepository,
	defaultIntervalM int,
) *Service {
	return &Service{
		idGenerator:      idGenerator,
		clock:            clock,
		trackedKeywords:  trackedKeywords,
		defaultIntervalM: defaultIntervalM,
	}
}

func (s *Service) AddKeyword(ctx context.Context, keyword string) error {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return fmt.Errorf("keyword is required")
	}

	now := s.clock.Now()
	return s.trackedKeywords.Create(ctx, domain.TrackedKeyword{
		ID:              s.idGenerator.Next(),
		Keyword:         keyword,
		IntervalMinutes: s.defaultIntervalM,
		TelegramEnabled: false,
		Status:          domain.TrackedKeywordStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
}

func (s *Service) PauseKeyword(ctx context.Context, keywordID string) error {
	return s.updateStatus(ctx, keywordID, domain.TrackedKeywordStatusPaused)
}

func (s *Service) ResumeKeyword(ctx context.Context, keywordID string) error {
	return s.updateStatus(ctx, keywordID, domain.TrackedKeywordStatusActive)
}

func (s *Service) ArchiveKeyword(ctx context.Context, keywordID string) error {
	return s.updateStatus(ctx, keywordID, domain.TrackedKeywordStatusArchived)
}

func (s *Service) UpdateThreshold(ctx context.Context, keywordID string, threshold *int64) error {
	if threshold != nil && *threshold <= 0 {
		return fmt.Errorf("threshold price must be > 0")
	}

	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}

	keyword.ThresholdPrice = threshold
	keyword.UpdatedAt = s.clock.Now()
	return s.trackedKeywords.Update(ctx, *keyword)
}

func (s *Service) UpdateInterval(ctx context.Context, keywordID string, intervalMinutes int) error {
	if intervalMinutes <= 0 {
		return fmt.Errorf("interval minutes must be > 0")
	}

	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}

	keyword.IntervalMinutes = intervalMinutes
	keyword.UpdatedAt = s.clock.Now()
	return s.trackedKeywords.Update(ctx, *keyword)
}

func (s *Service) UpdateBasicFilter(ctx context.Context, keywordID string, basicFilter *string) error {
	if basicFilter != nil {
		trimmed := strings.TrimSpace(*basicFilter)
		if trimmed == "" {
			basicFilter = nil
		} else {
			basicFilter = &trimmed
		}
	}

	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}

	keyword.BasicFilter = basicFilter
	keyword.UpdatedAt = s.clock.Now()
	return s.trackedKeywords.Update(ctx, *keyword)
}

func (s *Service) SetTelegramEnabled(ctx context.Context, keywordID string, enabled bool) error {
	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}

	keyword.TelegramEnabled = enabled
	keyword.UpdatedAt = s.clock.Now()
	return s.trackedKeywords.Update(ctx, *keyword)
}

func (s *Service) updateStatus(ctx context.Context, keywordID string, status domain.TrackedKeywordStatus) error {
	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}

	keyword.Status = status
	keyword.UpdatedAt = s.clock.Now()
	return s.trackedKeywords.Update(ctx, *keyword)
}
