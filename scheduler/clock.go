package scheduler

import "time"

// Clock supplies the current time. Tests inject a mock clock to drive
// schedulers deterministically.
type Clock interface {
	Now() time.Time
}

// SystemClock returns time.Now().
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// MockClock is a deterministic clock for tests. Advance with Add.
type MockClock struct {
	now time.Time
}

func NewMockClock(start time.Time) *MockClock { return &MockClock{now: start} }

func (m *MockClock) Now() time.Time { return m.now }

func (m *MockClock) Add(d time.Duration) { m.now = m.now.Add(d) }
