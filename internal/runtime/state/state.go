package state

import (
	"sync"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

type KeywordState struct {
	Running        bool
	NextEligibleAt time.Time
	LastAttemptAt  *time.Time
	LastFinishedAt *time.Time
	LastError      *string
	LastSuccessAt  *time.Time
}

type Store struct {
	mu     sync.RWMutex
	states map[string]KeywordState
}

func NewStore() *Store {
	return &Store{
		states: make(map[string]KeywordState),
	}
}

func (s *Store) EnsureKeywords(keywords []domain.TrackedKeyword) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, keyword := range keywords {
		if _, exists := s.states[keyword.ID]; !exists {
			s.states[keyword.ID] = KeywordState{}
		}
	}
}

func (s *Store) Snapshot(keywordID string) KeywordState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.states[keywordID]
}

type Summary struct {
	KeywordsTracked int
	RunningCount    int
	FailedKeywords  int
	LatestFailureAt *time.Time
	LatestFailure   *string
}

func (s *Store) Summary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := Summary{KeywordsTracked: len(s.states)}
	for _, keywordState := range s.states {
		if keywordState.Running {
			summary.RunningCount++
		}
		if keywordState.LastError != nil {
			summary.FailedKeywords++
			if isLaterTime(keywordState.LastFinishedAt, summary.LatestFailureAt) ||
				(summary.LatestFailureAt == nil && keywordState.LastAttemptAt != nil && isLaterTime(keywordState.LastAttemptAt, summary.LatestFailureAt)) {
				summary.LatestFailureAt = cloneTime(firstNonNilTime(keywordState.LastFinishedAt, keywordState.LastAttemptAt))
				summary.LatestFailure = cloneString(keywordState.LastError)
			}
		}
	}

	return summary
}

func (s *Store) IsEligible(keywordID string, now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[keywordID]
	if !ok {
		return true
	}

	if state.Running {
		return false
	}

	return state.NextEligibleAt.IsZero() || !now.Before(state.NextEligibleAt)
}

func (s *Store) MarkRunning(keywordID string, startedAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.states[keywordID]
	if current.Running {
		return false
	}

	current.Running = true
	current.LastAttemptAt = timePointer(startedAt)
	s.states[keywordID] = current
	return true
}

func (s *Store) MarkFinished(keywordID string, finishedAt time.Time, intervalMinutes int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.states[keywordID]
	current.Running = false
	current.LastFinishedAt = timePointer(finishedAt)
	current.NextEligibleAt = finishedAt.Add(time.Duration(intervalMinutes) * time.Minute)
	if err != nil {
		message := err.Error()
		current.LastError = &message
	} else {
		current.LastError = nil
		current.LastSuccessAt = timePointer(finishedAt)
	}
	s.states[keywordID] = current
}

func timePointer(value time.Time) *time.Time {
	v := value
	return &v
}

func isLaterTime(candidate, current *time.Time) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	return candidate.After(*current)
}

func firstNonNilTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
