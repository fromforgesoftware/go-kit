package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/app"
	"github.com/fromforgesoftware/go-kit/openapi"
)

// pickPort sets REST_ADDRESS to a free port so the kit's rest module
// can listen during the test. Returned func restores prior state.
func pickPort(t *testing.T) (addr string, restore func()) {
	t.Helper()
	prev, had := os.LookupEnv("REST_ADDRESS")
	// :0 picks any free port — but rest.FxModule reads the env at
	// boot and we can't introspect the actual port back through fx
	// without more plumbing. For these unit tests we just need a
	// valid address; :0 binds, and we won't be making HTTP calls
	// against it.
	require.NoError(t, os.Setenv("REST_ADDRESS", "127.0.0.1:0"))
	return "127.0.0.1:0", func() {
		if had {
			_ = os.Setenv("REST_ADDRESS", prev)
		} else {
			_ = os.Unsetenv("REST_ADDRESS")
		}
	}
}

func TestRun_Defaults_BootsAndStops(t *testing.T) {
	_, restore := pickPort(t)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cancel shortly after start so RunContext returns without waiting
	// for a real signal.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := app.RunContext(ctx,
		app.WithName("test-svc"),
		app.WithVersion("0.0.1"),
	)
	// Context-cancelled shutdown counts as clean here.
	assert.NoError(t, err)
}

func TestRun_InfoProvided_InjectableViaFx(t *testing.T) {
	_, restore := pickPort(t)
	defer restore()

	var seen app.Info
	captured := make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		<-captured
		// Let the fx graph finish starting before cancelling.
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := app.RunContext(ctx,
		app.WithName("info-svc"),
		app.WithVersion("9.9.9"),
		fx.Invoke(func(info app.Info) {
			seen = info
			close(captured)
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "info-svc", seen.Name)
	assert.Equal(t, "9.9.9", seen.Version)
}

func TestRun_WithOpenAPI_AddsSpecOpts(t *testing.T) {
	_, restore := pickPort(t)
	defer restore()

	// We can't easily inspect the spec without hitting HTTP, but we
	// can at least assert that the fx graph builds when WithOpenAPI
	// is provided — exercises the kit's rest.WithOpenAPI plumbing.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := app.RunContext(ctx,
		app.WithName("openapi-svc"),
		app.WithVersion("1.0.0"),
		app.WithOpenAPI(
			openapi.SpecTitle("Test"),
			openapi.SpecVersion("1.0.0"),
		),
	)
	assert.NoError(t, err)
}

func TestRun_WithoutHTTP_SkipsRestModule(t *testing.T) {
	// REST_ADDRESS not set on purpose — proves rest.FxModule didn't
	// boot. If it had, the missing env var would panic the kit's
	// envconfig at startup.
	_ = os.Unsetenv("REST_ADDRESS")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invoked := make(chan struct{})
	go func() {
		<-invoked
		cancel()
	}()

	err := app.RunContext(ctx,
		app.WithName("worker"),
		app.WithoutHTTP(),
		fx.Invoke(func() { close(invoked) }),
	)
	assert.NoError(t, err)
}

func TestRunWithConfig_PopulatesEnvAndProvidesPointer(t *testing.T) {
	_, restore := pickPort(t)
	defer restore()
	require.NoError(t, os.Setenv("TEST_GREETING", "hi"))
	defer func() { _ = os.Unsetenv("TEST_GREETING") }()

	type Config struct {
		Greeting string `envconfig:"TEST_GREETING" required:"true"`
	}

	var cfg Config
	captured := make(chan string, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		select {
		case <-captured:
		case <-ctx.Done():
		}
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := app.RunContextWithConfig(ctx, &cfg,
		app.WithName("config-svc"),
		fx.Invoke(func(c *Config) {
			captured <- c.Greeting
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "hi", cfg.Greeting)
}
