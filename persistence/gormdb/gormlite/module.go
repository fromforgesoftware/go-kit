package gormlite

import (
	"context"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
)

// FxModule provides a *gormdb.DBClient over the database file at path, a persistence.Transactioner
// on it, and closes the client on shutdown.
func FxModule(path string, cliOptions ...gormdb.Option) fx.Option {
	return fx.Module(
		"gormlite",
		fx.Provide(func(m monitoring.Monitor) (*gormdb.DBClient, error) {
			return NewClient(path, m, cliOptions...)
		}),
		fx.Provide(fx.Annotate(gormdb.NewTransactioner, fx.As(new(persistence.Transactioner)))),
		fx.Invoke(initLifecycle),
	)
}

func initLifecycle(lc fx.Lifecycle, db *gormdb.DBClient) error {
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return db.Close()
		},
	})

	return nil
}
