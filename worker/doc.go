// Package worker is forge's background-job runtime. It composes with
// app.RunWorker (sibling to app.Run with HTTP disabled) and provides
// three primitives:
//
//   - Cron: interval-based recurring jobs
//   - Queue[T]: typed pub/sub topics with pluggable backends
//   - Outbox: drain the kit's outbox table (planned follow-up)
//
// Wave 3 ships Cron + Queue against an in-memory backend. The Postgres
// LISTEN/NOTIFY and NATS JetStream backends slot behind the same
// QueueBackend interface in a follow-up.
//
//	// services/billing-worker/main.go
//	app.RunWorker(
//	    app.WithName("billing"),
//	    worker.FxModule(),
//	    worker.Cron(2*time.Hour, "daily-rollup", dailyRollup),
//	    worker.Queue[CreateInvoiceCmd]("invoices.create", createInvoice),
//	    internal.FxModule(),
//	)
package worker
