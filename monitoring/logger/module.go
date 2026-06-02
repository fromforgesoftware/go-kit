package logger

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
)

type Configuration struct {
	ServiceName string     `required:"true" envconfig:"SVC_NAME"`
	LogLevel    string     `required:"false" envconfig:"LOG_LEVEL" default:"info"`
	LoggerType  LoggerType `required:"false" envconfig:"LOGGER_TYPE" default:"zap"`
}

func FxModule() fx.Option {
	return fx.Module(
		"logger",
		fx.Provide(fx.Annotate(newLogger, fx.As(new(Logger)))),
	)
}

func newLogger() Logger {
	cfg := Configuration{}

	err := envconfig.Process("", &cfg)
	if err != nil {
		panic(err)
	}

	return New(
		WithType(LoggerType(cfg.LoggerType)),
		WithLevel(ParseLevel(cfg.LogLevel)),
	)
}
