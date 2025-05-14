package rabbit

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

var FXModule = fx.Module("rabbit",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterRabbitLifecycle),
)

func RegisterRabbitLifecycle(lc fx.Lifecycle, client *Rabbit, logger Logger, cfg Config) {
	wg := &sync.WaitGroup{}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)

			go func(logger Logger, cfg Config) {
				defer wg.Done()
				client.retryConnection(logger, cfg)
			}(logger, cfg)

			return nil
		},
		OnStop: func(ctx context.Context) error {

			client.gracefulShutdown()

			wg.Wait()
			return nil
		},
	})
}

func (rb *Rabbit) gracefulShutdown() {
	close(rb.shutdownSignal)
	rb.mu.Lock()

	rb.logger.Info("closing rabbit channel...", nil, nil)

	if rb.Channel != nil {
		if err := rb.Channel.Close(); err != nil {
			rb.logger.Error("error in closing rabbit channel", err, nil)
			return
		}
	}
	if rb.conn != nil && !rb.conn.IsClosed() {
		if err := rb.conn.Close(); err != nil {
			rb.logger.Error("error in closing rabbit connection", err, nil)
			return
		}
	}
	rb.mu.Unlock()
}
