package workflow_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/workflow"
)

type orderState string
type orderEvent string

const (
	sCreated   orderState = "created"
	sPaid      orderState = "paid"
	sShipped   orderState = "shipped"
	sCancelled orderState = "cancelled"

	eAuthorise orderEvent = "authorise"
	eShip      orderEvent = "ship"
	eCancel    orderEvent = "cancel"
)

func newSpec() workflow.Spec[orderState, orderEvent] {
	return workflow.Spec[orderState, orderEvent]{
		Initial: sCreated,
		Transitions: []workflow.Edge[orderState, orderEvent]{
			{From: sCreated, Event: eAuthorise, To: sPaid},
			{From: sPaid, Event: eShip, To: sShipped},
			{From: sCreated, Event: eCancel, To: sCancelled},
			{From: sPaid, Event: eCancel, To: sCancelled},
		},
	}
}

func TestStartAndState(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	s, err := mgr.State(ctx, "o1")
	require.NoError(t, err)
	assert.Equal(t, sCreated, s)
}

func TestSendAdvancesState(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k1"))
	s, _ := mgr.State(ctx, "o1")
	assert.Equal(t, sPaid, s)
}

func TestSendIdempotent(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k1"))
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k1")) // dupe — no-op
	hist, _ := workflow.NewMemoryStore().History(ctx, "o1")
	_ = hist
	s, _ := mgr.State(ctx, "o1")
	assert.Equal(t, sPaid, s)
}

func TestIllegalTransition(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	err := mgr.Send(ctx, "o1", eShip, "k1")
	assert.ErrorIs(t, err, workflow.ErrIllegalTransition)
}

func TestStartTwiceFails(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	err := mgr.Start(ctx, "o1")
	assert.ErrorIs(t, err, workflow.ErrAlreadyStarted)
}

func TestResumeFromStore(t *testing.T) {
	store := workflow.NewMemoryStore()
	mgr := workflow.NewManager(newSpec(), store)
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k1"))

	// New manager instance sharing the same store — simulates restart.
	mgr2 := workflow.NewManager(newSpec(), store)
	require.NoError(t, mgr2.Resume(ctx))
	s, err := mgr2.State(ctx, "o1")
	require.NoError(t, err)
	assert.Equal(t, sPaid, s)
	require.NoError(t, mgr2.Send(ctx, "o1", eShip, "k2"))
	s, _ = mgr2.State(ctx, "o1")
	assert.Equal(t, sShipped, s)
}

func TestCompensation(t *testing.T) {
	store := workflow.NewMemoryStore()
	var compensated int32
	spec := newSpec()
	spec.Compensators = map[orderState]func(context.Context, workflow.WorkflowID) error{
		sPaid: func(_ context.Context, _ workflow.WorkflowID) error {
			atomic.AddInt32(&compensated, 1)
			return nil
		},
	}
	mgr := workflow.NewManager(spec, store)
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k1"))
	require.NoError(t, mgr.Compensate(ctx, "o1", sCreated))
	assert.Equal(t, int32(1), atomic.LoadInt32(&compensated))
	s, _ := mgr.State(ctx, "o1")
	assert.Equal(t, sCreated, s)
}

func TestStateUnknownInstance(t *testing.T) {
	mgr := workflow.NewManager(newSpec(), workflow.NewMemoryStore())
	_, err := mgr.State(context.Background(), "missing")
	assert.ErrorIs(t, err, workflow.ErrUnknownInstance)
}

func TestGuardBlocksTransition(t *testing.T) {
	spec := newSpec()
	allow := false
	for i, t := range spec.Transitions {
		if t.From == sCreated && t.Event == eAuthorise {
			spec.Transitions[i].Guard = func(_ context.Context, _ workflow.WorkflowID) bool { return allow }
		}
	}
	mgr := workflow.NewManager(spec, workflow.NewMemoryStore())
	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx, "o1"))
	err := mgr.Send(ctx, "o1", eAuthorise, "k1")
	assert.ErrorIs(t, err, workflow.ErrIllegalTransition)
	allow = true
	require.NoError(t, mgr.Send(ctx, "o1", eAuthorise, "k2"))
	s, _ := mgr.State(ctx, "o1")
	assert.Equal(t, sPaid, s)
}
