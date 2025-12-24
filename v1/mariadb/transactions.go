package mariadb

import (
	"context"

	"gorm.io/gorm"
)

// cloneWithTx returns a shallow copy of MariaDB with tx as the DB Client.
// This internal helper method creates a new MariaDB instance that shares most
// properties with the original but uses the provided transaction as its database Client.
// It enables transaction-scoped operations while maintaining the connection monitoring
// and safety features of the MariaDB wrapper.
func (m *MariaDB) cloneWithTx(tx *gorm.DB) *MariaDB {
	return &MariaDB{
		Client:          tx,
		cfg:             m.cfg,
		mu:              m.mu, // shared mutex is fine
		shutdownSignal:  m.shutdownSignal,
		retryChanSignal: m.retryChanSignal,
	}
}

// Transaction executes the given function within a database transaction.
// It creates a transaction-specific MariaDB instance and passes it to the provided function.
// If the function returns an error, the transaction is rolled back; otherwise, it's committed.
//
// This method provides a clean way to execute multiple database operations as a single
// atomic unit, with automatic handling of commit/rollback based on the execution result.
//
// Returns a GORM error if the transaction fails or the error returned by the callback function.
//
// Example usage:
//
//	err := db.Transaction(ctx, func(txDB *MariaDB) error {
//		if err := txDB.Create(ctx, user); err != nil {
//			return err
//		}
//		return txDB.Create(ctx, userProfile)
//	})
func (m *MariaDB) Transaction(ctx context.Context, fn func(db *MariaDB) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbWithTx := m.cloneWithTx(tx)
		return fn(dbWithTx)
	})
}
