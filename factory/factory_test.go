package factory_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/factory"
)

type widget struct{ Name string }

func TestRegistryRoundTrip(t *testing.T) {
	r := factory.New[widget]()
	require.NoError(t, r.Register("a", widget{Name: "A"}))
	require.NoError(t, r.Register("b", widget{Name: "B"}))
	w, ok := r.Get("a")
	require.True(t, ok)
	assert.Equal(t, "A", w.Name)
	assert.Equal(t, []string{"a", "b"}, r.Keys())
}

func TestRegistryDuplicate(t *testing.T) {
	r := factory.New[widget]()
	require.NoError(t, r.Register("a", widget{}))
	err := r.Register("a", widget{})
	assert.ErrorIs(t, err, factory.ErrDuplicate)
}

func TestRegistryMustRegisterPanicsOnDuplicate(t *testing.T) {
	r := factory.New[widget]()
	r.MustRegister("a", widget{})
	assert.Panics(t, func() { r.MustRegister("a", widget{}) })
}

func TestRegistryFreezeBlocksWrites(t *testing.T) {
	r := factory.New[widget]()
	require.NoError(t, r.Register("a", widget{}))
	r.Freeze()
	assert.True(t, r.IsFrozen())
	err := r.Register("b", widget{})
	assert.ErrorIs(t, err, factory.ErrFrozen)
	// Reads still work.
	_, ok := r.Get("a")
	assert.True(t, ok)
}

func TestRegistryConcurrentReadsAfterFreeze(t *testing.T) {
	r := factory.New[widget]()
	for _, k := range []string{"a", "b", "c"} {
		require.NoError(t, r.Register(k, widget{Name: k}))
	}
	r.Freeze()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = r.Get("a")
				_, _ = r.Get("b")
				_, _ = r.Get("c")
			}
		}()
	}
	wg.Wait()
}

func TestBuildersInvokeWithParams(t *testing.T) {
	b := factory.NewBuilders[string, widget]()
	require.NoError(t, b.Register("named", func(name string) (widget, error) {
		return widget{Name: name}, nil
	}))
	w, err := b.Build("named", "from-param")
	require.NoError(t, err)
	assert.Equal(t, "from-param", w.Name)
}

func TestBuildersErrorPropagates(t *testing.T) {
	b := factory.NewBuilders[int, widget]()
	bad := errors.New("nope")
	require.NoError(t, b.Register("k", func(_ int) (widget, error) { return widget{}, bad }))
	_, err := b.Build("k", 1)
	assert.ErrorIs(t, err, bad)
}

func TestBuildersUnknownKey(t *testing.T) {
	b := factory.NewBuilders[int, widget]()
	_, err := b.Build("missing", 0)
	assert.ErrorIs(t, err, factory.ErrNotFound)
}

func TestBuildersFreezeBlocks(t *testing.T) {
	b := factory.NewBuilders[int, widget]()
	require.NoError(t, b.Register("k", func(_ int) (widget, error) { return widget{}, nil }))
	b.Freeze()
	assert.True(t, b.Has("k"))
	err := b.Register("x", func(_ int) (widget, error) { return widget{}, nil })
	assert.ErrorIs(t, err, factory.ErrFrozen)
}
