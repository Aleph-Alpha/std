# Plan: Revert std to v0.14.0 Interface Design

## Current State (v0.15.0 / main)

- ❌ `v1/database/` package exists with shared interface
- ⚠️ `postgres.Client` exists but marked DEPRECATED
- ⚠️ `mariadb.Client` exists but marked DEPRECATED
- ❌ Documentation says "use database.Client"

## Target State (v0.14.0)

- ✅ NO `v1/database/` package
- ✅ `postgres.Client` interface (NOT deprecated)
- ✅ `mariadb.Client` interface (NOT deprecated)
- ✅ Each package has its own interface
- ✅ Applications define their own abstraction if needed

## Why v0.14.0 Was Better

1. **No circular imports** - No database package to create cycles
2. **Each package owns its interface** - postgres.Client in postgres, mariadb.Client in mariadb
3. **No false promises** - Don't claim they're the same when they're not
4. **Industry standard** - AWS SDK, k8s, GORM all work this way
5. **Application control** - Apps define their own abstraction layer

## Changes Needed in std

### 1. Delete `v1/database/` Package
```bash
rm -rf v1/database/
```

### 2. Update `v1/postgres/interface.go`
- Remove "DEPRECATED" warnings
- Remove references to `database.Client`
- Update doc comments:
  - "This package provides PostgreSQL operations"
  - NOT "implements database.Client"

### 3. Update `v1/mariadb/interface.go`
- Remove "DEPRECATED" warnings
- Remove references to `database.Client`
- Update doc comments:
  - "This package provides MariaDB operations"
  - NOT "implements database.Client"

### 4. Update Documentation
- `docs/v1/postgres.md` - Remove database.Client references
- `docs/v1/mariadb.md` - Remove database.Client references
- Add migration guide for v0.15.0 → v0.16.0 users

## Git Strategy

### Option A: Revert Commit
```bash
git checkout main
git revert c009db7  # Revert the unified interface commit
git revert b179cfe  # Revert the release commit
```

### Option B: Cherry-pick from v0.14.0 (RECOMMENDED)
```bash
# Create new branch
git checkout -b feat/revert-to-separate-interfaces

# Get the interface files from v0.14.0
git show v0.14.0:v1/postgres/interface.go > v1/postgres/interface.go
git show v0.14.0:v1/mariadb/interface.go > v1/mariadb/interface.go

# Delete database package
rm -rf v1/database/

# Commit
git add -A
git commit -m "feat!: revert to separate interfaces per database package

BREAKING CHANGE: Removed v1/database package and unified interface.

Rationale:
- v0.15.0's unified interface created more problems than it solved
- Circular import issues when trying to use the interface
- False promise of compatibility (Go's type system doesn't allow covariance)
- Industry standard is concrete implementations (AWS SDK, k8s, GORM)

Changes:
- Deleted: v1/database/ (entire package)
- Restored: postgres.Client interface (no longer deprecated)
- Restored: mariadb.Client interface (no longer deprecated)
- Updated: Documentation to reflect separate interfaces

Migration from v0.15.0:
- If using database.Client: Define your own interface in your app
- If using postgres.Client/mariadb.Client: No changes needed
- Applications should implement their own abstraction layer using adapters

This returns us to the v0.14.0 design, which was correct."
```

## Changes Needed in pharia-data-api

**GOOD NEWS:** We already have the adapter pattern implemented!

Our current code:
- ✅ `client.Client` interface (our own)
- ✅ `client.QueryBuilder` interface (our own)
- ✅ `PostgresAdapter` wrapping `*postgres.Postgres`
- ✅ `MariaDBAdapter` wrapping `*mariadb.MariaDB`

**NO CHANGES NEEDED** - Our adapters already use the concrete types from std!

### Why Our Code Still Works

Our adapter uses:
```go
type PostgresAdapter struct {
    pg *postgres.Postgres  // Concrete type!
}

func (a *PostgresAdapter) Transaction(ctx, fn) error {
    return a.pg.Transaction(ctx, func(pgTx postgres.Client) error {
        // ✅ This will still work because postgres.Client exists in v0.14.0
    })
}
```

The only difference:
- v0.15.0: `postgres.Client` marked DEPRECATED
- v0.14.0: `postgres.Client` NOT deprecated

**Our code works with both!**

## Testing Plan

### 1. Update std Library
```bash
cd std
git checkout -b feat/revert-to-separate-interfaces
# Apply changes above
go build ./v1/postgres/... ./v1/mariadb/...
go test ./v1/postgres/... ./v1/mariadb/...
```

### 2. Test pharia-data-api with Updated std
```bash
cd pharia-data-api
go mod edit -replace github.com/Aleph-Alpha/std=../std
go build ./cmd/...
go test ./internal/infrastructure/database/...
```

### 3. Verify Everything Works
- ✅ std builds without database package
- ✅ pharia-data-api builds with adapters
- ✅ Tests pass
- ✅ No import errors

## Benefits

1. **Simpler std** - No confusing database package
2. **No circular imports** - Clean dependency graph
3. **Clear ownership** - postgres owns postgres.Client
4. **Industry standard** - Direct concrete types
5. **Application control** - Apps define abstraction (like we did!)

## Timeline

1. ✅ Check v0.14.0 design (DONE)
2. Create branch in std
3. Revert to v0.14.0 interface design
4. Test std builds
5. Test pharia-data-api with updated std
6. Create PR in std
7. Release as v0.16.0 (breaking change)

## Version Strategy

- v0.14.0: Separate interfaces ✅ (good)
- v0.15.0: Unified interface ❌ (bad - created problems)
- v0.16.0: Separate interfaces ✅ (revert to good design)

This is a valid strategy - sometimes you need to try something to realize it was wrong!

## Next Step

Should we proceed with Option B (cherry-pick from v0.14.0)?
