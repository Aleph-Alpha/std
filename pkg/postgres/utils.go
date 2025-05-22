package postgres

import (
	"gorm.io/gorm"
)

// DB returns the underlying GORM DB client instance.
// This method provides direct access to the database connection while
// maintaining thread safety through a read lock.
//
// Use this method when you need to perform operations not covered by
// the wrapper methods or when you need to access specific GORM functionality.
// Note that direct usage bypasses some of the safety mechanisms, so use it with care.
func (p *Postgres) DB() *gorm.DB {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}
