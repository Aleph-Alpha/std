package postgres

import (
	"context"
	"gorm.io/gorm"
)

// Query provides a flexible way to build complex queries
// It returns a QueryBuilder which can be used to chain query methods
func (p *Postgres) Query(ctx context.Context) *QueryBuilder {
	p.mu.RLock() // Will be released when Done() is called
	return &QueryBuilder{
		db:      p.client.WithContext(ctx),
		release: p.mu.RUnlock,
	}
}

// QueryBuilder provides a fluent interface for building complex queries
type QueryBuilder struct {
	db      *gorm.DB
	release func() // Function to release the mutex lock
}

// Select specifies fields to be selected
func (qb *QueryBuilder) Select(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Select(query, args...)
	return qb
}

// Where adds a where condition
func (qb *QueryBuilder) Where(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Where(query, args...)
	return qb
}

// Or adds an OR condition
func (qb *QueryBuilder) Or(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Or(query, args...)
	return qb
}

// Not adds a NOT condition
func (qb *QueryBuilder) Not(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Not(query, args...)
	return qb
}

// Joins adds a JOIN clause
func (qb *QueryBuilder) Joins(query string, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Joins(query, args...)
	return qb
}

// LeftJoin adds a LEFT JOIN clause
func (qb *QueryBuilder) LeftJoin(query string, args ...interface{}) *QueryBuilder {
	joinClause := "LEFT JOIN " + query
	qb.db = qb.db.Joins(joinClause, args...)
	return qb
}

// RightJoin adds a RIGHT JOIN clause
func (qb *QueryBuilder) RightJoin(query string, args ...interface{}) *QueryBuilder {
	joinClause := "RIGHT JOIN " + query
	qb.db = qb.db.Joins(joinClause, args...)
	return qb
}

// Preload preloads associations
func (qb *QueryBuilder) Preload(query string, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Preload(query, args...)
	return qb
}

// Group adds a GROUP BY clause
func (qb *QueryBuilder) Group(query string) *QueryBuilder {
	qb.db = qb.db.Group(query)
	return qb
}

// Having added a HAVING clause
func (qb *QueryBuilder) Having(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Having(query, args...)
	return qb
}

// Order adds an ORDER BY clause
func (qb *QueryBuilder) Order(value interface{}) *QueryBuilder {
	qb.db = qb.db.Order(value)
	return qb
}

// Limit sets the limit for the query
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.db = qb.db.Limit(limit)
	return qb
}

// Offset sets the offset for the query
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.db = qb.db.Offset(offset)
	return qb
}

// Raw executes raw SQL
func (qb *QueryBuilder) Raw(sql string, values ...interface{}) *QueryBuilder {
	qb.db = qb.db.Raw(sql, values...)
	return qb
}

// Model specifies the model to use
func (qb *QueryBuilder) Model(value interface{}) *QueryBuilder {
	qb.db = qb.db.Model(value)
	return qb
}

// Scan scans the result into dest
func (qb *QueryBuilder) Scan(dest interface{}) error {
	defer qb.release()
	return qb.db.Scan(dest).Error
}

// Find finds records that match the given conditions
func (qb *QueryBuilder) Find(dest interface{}) error {
	defer qb.release()
	return qb.db.Find(dest).Error
}

// First finds the first record that matches the given conditions
func (qb *QueryBuilder) First(dest interface{}) error {
	defer qb.release()
	return qb.db.First(dest).Error
}

// Last finds the last record that matches the given conditions
func (qb *QueryBuilder) Last(dest interface{}) error {
	defer qb.release()
	return qb.db.Last(dest).Error
}

// Count counts records that match the given conditions
func (qb *QueryBuilder) Count(count *int64) error {
	defer qb.release()
	return qb.db.Count(count).Error
}

// Updates updates records with the given values
func (qb *QueryBuilder) Updates(values interface{}) error {
	defer qb.release()
	return qb.db.Updates(values).Error
}

// Delete deletes records that match the given conditions
func (qb *QueryBuilder) Delete(value interface{}) error {
	defer qb.release()
	return qb.db.Delete(value).Error
}

// Pluck queries a single column and scans it into dest
func (qb *QueryBuilder) Pluck(column string, dest interface{}) error {
	defer qb.release()
	return qb.db.Pluck(column, dest).Error
}

// Distinct specifies DISTINCT clause
func (qb *QueryBuilder) Distinct(args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Distinct(args...)
	return qb
}

// Done releases the mutex lock without executing the query
// Useful when you want to cancel a query building chain
func (qb *QueryBuilder) Done() {
	qb.release()
}
