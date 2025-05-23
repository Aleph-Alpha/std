package tracer

import (
	"context"
	"go.uber.org/fx"
)

var FXModule = fx.Module("tracer",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterTracerLifecycle),
)

func RegisterTracerLifecycle(lc fx.Lifecycle, tracer *Tracer) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			tracer.logger.Info("shutting down tracer tracer...", nil, nil)
			if tracer.tracer == nil {
				tracer.logger.Warn("tracer was nil during shutdown", nil, nil)
				return nil
			}
			return tracer.tracer.Shutdown(ctx)
		},
	})
}
