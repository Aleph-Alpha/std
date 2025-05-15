package postgres

import (
	"context"
	"gorm.io/gorm"
)

// Transaction executes the given function within a database transaction
func (p *Postgres) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Transaction(fn)
}
