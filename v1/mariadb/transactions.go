package mariadb

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// cloneWithTx returns a shallow copy of MariaDB with tx as the DB Client.
// This internal helper method creates a new MariaDB instance that shares most
// properties with the original but uses the provided transaction as its database Client.
// It enables transaction-scoped operations while maintaining the connection monitoring
// and safety features of the MariaDB wrapper.
func (m *MariaDB) cloneWithTx(tx *gorm.DB) *MariaDB {
	db := &MariaDB{
		cfg:      m.cfg,
		observer: m.observer,
		logger:   m.logger,
	}
	// Avoid sharing lifecycle channels with the parent; tx clients should not be able
	// to shut down monitoring goroutines.
	db.client.Store(tx)
	return db
}

// Transaction executes the given function within a database transaction.
// It creates a transaction-specific MariaDB instance and passes it as Client interface.
// If the function returns an error, the transaction is rolled back; otherwise, it's committed.
//
// This method provides a clean way to execute multiple database operations as a single
// atomic unit, with automatic handling of commit/rollback based on the execution result.
//
// Returns a GORM error if the transaction fails or the error returned by the callback function.
//
// Example usage:
//
//	err := db.Transaction(ctx, func(tx Client) error {
//		if err := tx.Create(ctx, user); err != nil {
//			return err
//		}
//		return tx.Create(ctx, userProfile)
//	})
func (m *MariaDB) Transaction(ctx context.Context, fn func(tx Client) error) error {
	start := time.Now()
	db := m.DB().WithContext(ctx)
	err := db.Transaction(func(txDB *gorm.DB) error {
		dbWithTx := m.cloneWithTx(txDB)
		return fn(dbWithTx) // Pass as Client interface
	})
	m.observeOperation("transaction", "", "", time.Since(start), err, 0, nil)
	return err
}
