package firebase

import (
	firebaseAuth "firebase.google.com/go/v4/auth"
	"go.uber.org/fx"
)

func FxModule() fx.Option {
	return fx.Module(
		"firebase",
		fx.Provide(
			fx.Annotate(NewClient, fx.As(new(Client))),
			func(c Client) *firebaseAuth.Client {
				return c.AuthClient()
			},
		),
	)
}
