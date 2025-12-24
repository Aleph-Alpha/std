package postgres

import (
	"context"

	"gorm.io/gorm"
)

// cloneWithTx returns a shallow copy of Postgres with tx as the DB Client.
// This internal helper method creates a new Postgres instance that shares most
// properties with the original but uses the provided transaction as its database Client.
// It enables transaction-scoped operations while maintaining the connection monitoring
// and safety features of the Postgres wrapper.
func (p *Postgres) cloneWithTx(tx *gorm.DB) *Postgres {
	return &Postgres{
		Client:          tx,
		cfg:             p.cfg,
		mu:              p.mu, // shared mutex is fine
		shutdownSignal:  p.shutdownSignal,
		retryChanSignal: p.retryChanSignal,
	}
}

// Transaction executes the given function within a database transaction.
// It creates a transaction-specific Postgres instance and passes it as Client interface.
// If the function returns an error, the transaction is rolled back; otherwise, it's committed.
//
// This method provides a clean way to execute multiple database operations as a single
// atomic unit, with automatic handling of commit/rollback based on the execution result.
//
// Returns a GORM error if the transaction fails or the error returned by the callback function.
//
// Example usage:
//
//	err := pg.Transaction(ctx, func(tx Client) error {
//		if err := tx.Create(ctx, user); err != nil {
//			return err
//		}
//		return tx.Create(ctx, userProfile)
//	})
func (p *Postgres) Transaction(ctx context.Context, fn func(tx Client) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Client.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		pgWithTx := p.cloneWithTx(txDB)
		return fn(pgWithTx) // Pass as Client interface
	})
}
