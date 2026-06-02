package httpclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/httpclient"
	"github.com/fromforgesoftware/go-kit/monitoring"
)

func TestFxModule_InjectsNamedClient(t *testing.T) {
	t.Setenv("SVC_NAME", "httpclient-test")
	type deps struct {
		fx.In
		Stripe *httpclient.Client `name:"stripe"`
	}

	var got *httpclient.Client
	app := fx.New(
		fx.NopLogger,
		monitoring.FxModule(),
		httpclient.FxModule("stripe",
			httpclient.WithBaseURL("https://api.stripe.com"),
		),
		fx.Invoke(func(d deps) {
			got = d.Stripe
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Start(ctx))
	defer func() { _ = app.Stop(ctx) }()
	assert.NotNil(t, got, "stripe client should be injected")
}

func TestFxModule_TwoNamedClientsCoexist(t *testing.T) {
	t.Setenv("SVC_NAME", "httpclient-test")
	type deps struct {
		fx.In
		Stripe *httpclient.Client `name:"stripe"`
		Slack  *httpclient.Client `name:"slack"`
	}
	var d deps
	app := fx.New(
		fx.NopLogger,
		monitoring.FxModule(),
		httpclient.FxModule("stripe", httpclient.WithBaseURL("https://api.stripe.com")),
		httpclient.FxModule("slack", httpclient.WithBaseURL("https://slack.com/api")),
		fx.Populate(&d),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Start(ctx))
	defer func() { _ = app.Stop(ctx) }()
	assert.NotNil(t, d.Stripe)
	assert.NotNil(t, d.Slack)
	assert.NotSame(t, d.Stripe, d.Slack, "named clients are distinct instances")
}
