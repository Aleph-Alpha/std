package postgres

import (
	"context"
	"gorm.io/gorm"
)

// cloneWithTx returns a shallow copy of Postgres with tx as the DB client
func (p *Postgres) cloneWithTx(tx *gorm.DB) *Postgres {
	return &Postgres{
		client:          tx,
		cfg:             p.cfg,
		logger:          p.logger,
		mu:              p.mu, // shared mutex is fine
		shutdownSignal:  p.shutdownSignal,
		retryChanSignal: p.retryChanSignal,
	}
}

// Transaction executes the given Postgres method(s) in a transaction
func (p *Postgres) Transaction(ctx context.Context, fn func(pg *Postgres) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pgWithTx := p.cloneWithTx(tx)
		return fn(pgWithTx)
	})
}
