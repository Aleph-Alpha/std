package postgres

import (
	"context"
)

// Find finds records that match the given conditions
func (p *Postgres) Find(ctx context.Context, dest interface{}, conditions ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Find(dest, conditions...).Error
}

// First finds the first record that matches the given conditions
func (p *Postgres) First(ctx context.Context, dest interface{}, conditions ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).First(dest, conditions...).Error
}

// Create creates a new record
func (p *Postgres) Create(ctx context.Context, value interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Create(value).Error
}

// Save updates a record
func (p *Postgres) Save(ctx context.Context, value interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Save(value).Error
}

// Update updates records that match the given condition
// model should be the model type (e.g., &User{})
// attrs should be a map, struct, or name/value pair to update
func (p *Postgres) Update(ctx context.Context, model interface{}, attrs interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Model(model).Updates(attrs).Error
}

// UpdateColumn For updating with individual field-value pairs, provide a separate method
func (p *Postgres) UpdateColumn(ctx context.Context, model interface{}, columnName string, value interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Model(model).Update(columnName, value).Error
}

// UpdateColumns For updating multiple columns with name/value pairs
func (p *Postgres) UpdateColumns(ctx context.Context, model interface{}, columnValues map[string]interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Model(model).Updates(columnValues).Error
}

// Delete deletes records that match the given conditions
func (p *Postgres) Delete(ctx context.Context, value interface{}, conditions ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Delete(value, conditions...).Error
}

// Exec executes raw SQL
func (p *Postgres) Exec(ctx context.Context, sql string, values ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Exec(sql, values...).Error
}

// Count counts records that match the given conditions
func (p *Postgres) Count(ctx context.Context, model interface{}, count *int64, conditions ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Model(model).Where(conditions[0], conditions[1:]...).Count(count).Error
}

// UpdateWhere updates records that match the given condition
func (p *Postgres) UpdateWhere(ctx context.Context, model interface{}, attrs interface{}, condition string, args ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client.WithContext(ctx).Model(model).Where(condition, args...).Updates(attrs).Error
}
