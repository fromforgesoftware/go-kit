package repository

import "context"

type LockLevel string

const (
	LockLevelRow LockLevel = "ROW"
)

type LockMode string

const (
	LockModeExclusive LockMode = "EXCLUSIVE"
	LockModeShare     LockMode = "SHARE"
)

// Lock defines the interface for locking mechanisms.
type Lock interface {
	Modes() []LockMode
	Level() LockLevel
	Contains(mode LockMode) bool
}

type lock struct {
	lvl   LockLevel
	modes []LockMode
}

func (l *lock) Modes() []LockMode {
	return l.modes
}

func (l *lock) Level() LockLevel {
	return l.lvl
}

func (l *lock) Contains(mode LockMode) bool {
	for _, m := range l.modes {
		if m == mode {
			return true
		}
	}
	return false
}

// contextKeyType is a type for context key related to locking.
type contextKeyType int

const lockCtxKey contextKeyType = iota

// WithLockingCtx sets the lock context with the provided lock level and modes.
func WithLockingCtx(ctx context.Context, lockLevel LockLevel, lockModes ...LockMode) context.Context {
	return context.WithValue(ctx, lockCtxKey, &lock{lvl: lockLevel, modes: lockModes})
}

// LockFromCtx retrieves the lock from the context.
func LockFromCtx(ctx context.Context) Lock {
	lock := ctx.Value(lockCtxKey)
	if lock == nil {
		return nil
	}

	return lock.(Lock)
}
