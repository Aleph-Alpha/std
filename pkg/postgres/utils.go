package postgres

import (
	"gorm.io/gorm"
)

// DB returns the underlying GORM DB client
// This is for cases where direct access to GORM is needed
func (p *Postgres) DB() *gorm.DB {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}
