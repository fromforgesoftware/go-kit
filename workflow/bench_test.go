package workflow_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/fromforgesoftware/go-kit/workflow"
)

func BenchmarkSend(b *testing.B) {
	store := workflow.NewMemoryStore()
	mgr := workflow.NewManager(newSpec(), store)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := workflow.WorkflowID("o" + strconv.Itoa(i))
		_ = mgr.Start(ctx, id)
		_ = mgr.Send(ctx, id, eAuthorise, workflow.Idempotency("k"+strconv.Itoa(i)))
	}
}

func BenchmarkSendSingleInstance(b *testing.B) {
	store := workflow.NewMemoryStore()
	spec := workflow.Spec[orderState, orderEvent]{
		Initial: sCreated,
		Transitions: []workflow.Edge[orderState, orderEvent]{
			{From: sCreated, Event: eAuthorise, To: sPaid},
			{From: sPaid, Event: eAuthorise, To: sCreated}, // ping-pong for the bench
		},
	}
	mgr := workflow.NewManager(spec, store)
	ctx := context.Background()
	_ = mgr.Start(ctx, "o1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.Send(ctx, "o1", eAuthorise, workflow.Idempotency("k"+strconv.Itoa(i)))
	}
}

func BenchmarkState(b *testing.B) {
	store := workflow.NewMemoryStore()
	mgr := workflow.NewManager(newSpec(), store)
	ctx := context.Background()
	_ = mgr.Start(ctx, "o1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.State(ctx, "o1")
	}
}
