package postgres

// Migrate runs database migrations for the provided models
func (p *Postgres) Migrate(models ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.client.AutoMigrate(models...)
}
