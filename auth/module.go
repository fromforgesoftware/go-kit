package auth

import (
	"net/http"

	"go.uber.org/fx"
	"google.golang.org/grpc/metadata"
)

func FxModule() fx.Option {
	return fx.Module(
		"auth",
		fx.Provide(
			fx.Annotate(NewTokenContextInjector, fx.As(new(ContextInjector))),
			fx.Annotate(NewHTTPTokenExtractor, fx.As(new(TokenExtractor[*http.Request]))),
			fx.Annotate(NewGrpcTokenExtractor, fx.As(new(TokenExtractor[metadata.MD]))),
		),
	)
}
