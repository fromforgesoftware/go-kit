package jwt

import (
	"os"

	"go.uber.org/fx"
)

func FxModule() fx.Option {
	return fx.Module("jwt",
		fx.Provide(
			fx.Annotate(
				NewHMACIssuerFromEnv,
				fx.As(new(Issuer)),
				fx.As(new(Validator)),
				fx.As(new(IssuerValidator)),
			),
		),
	)
}

func NewHMACIssuerFromEnv() (*hmacIssuer, error) {
	secret := os.Getenv("HMAC_SECRET")
	if secret == "" {
		// Default for dev/test if not set
		secret = "default-dev-secret-do-not-use-in-prod"
	}
	return NewHMACIssuer(secret)
}
