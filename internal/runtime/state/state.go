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

func (s *Store) MarkFinished(keywordID string, finishedAt time.Time, intervalMinutes int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.states[keywordID]
	current.Running = false
	current.LastFinishedAt = timePointer(finishedAt)
	current.NextEligibleAt = finishedAt.Add(time.Duration(intervalMinutes) * time.Minute)
	s.states[keywordID] = current
}

func timePointer(value time.Time) *time.Time {
	v := value
	return &v
}
