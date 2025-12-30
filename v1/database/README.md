# Database Package

Unified interface for SQL database operations across PostgreSQL, MariaDB/MySQL, and future databases.

## Quick Start

### With FX Dependency Injection (Recommended)

```go
import (
    "go.uber.org/fx"
    "github.com/Aleph-Alpha/std/v1/database"
    "github.com/Aleph-Alpha/std/v1/postgres"
)

app := fx.New(
    database.FXModule,
    
    // Choose your database
    fx.Provide(func() database.Config {
        return database.PostgresConfig(postgres.Config{
            Connection: postgres.Connection{
                Host: "localhost",
                Port: 5432,
                User: "myuser",
                Password: "mypass",
                DbName: "mydb",
            },
        })
    }),
    
    // Your application receives database.Client
    fx.Invoke(func(db database.Client) {
        // Use db...
    }),
)
```

### Without FX

```go
import (
    "github.com/Aleph-Alpha/std/v1/database"
    "github.com/Aleph-Alpha/std/v1/postgres"
    "github.com/Aleph-Alpha/std/v1/mariadb"
)

// Application code depends on the interface
type UserService struct {
    db database.Client
}

// Select implementation via configuration
func NewDB(config Config) (database.Client, error) {
    switch config.DBType {
    case "postgres":
        return postgres.NewPostgres(config.Postgres)
    case "mariadb":
        return mariadb.NewMariaDB(config.MariaDB)
    default:
        return nil, fmt.Errorf("unsupported database: %s", config.DBType)
    }
}
```

## Switching Databases

To switch from PostgreSQL to MariaDB, just change the config:

```go
// From PostgreSQL
fx.Provide(func() database.Config {
    return database.PostgresConfig(postgres.Config{...})
})

// To MariaDB
fx.Provide(func() database.Config {
    return database.MariaDBConfig(mariadb.Config{...})
})
```

**No application code changes needed!**

## Benefits

- **True database-agnosticism**: Switch databases with configuration, not code changes
- **Interface compatibility**: Same methods work across PostgreSQL and MariaDB
- **Easy testing**: Mock the interface for unit tests
- **Future-proof**: Ready for additional databases (SQLite, CockroachDB, etc.)
- **FX Integration**: First-class support for dependency injection

## Implementations

| Database | Package | Implementation |
|----------|---------|----------------|
| PostgreSQL | `v1/postgres` | `*postgres.Postgres` |
| MariaDB/MySQL | `v1/mariadb` | `*mariadb.MariaDB` |

## Key Methods

### CRUD Operations
- `Find`, `First`, `Create`, `Save`, `Update`, `Delete`, `Count`

### Query Builder
- `Query(ctx).Where(...).Order(...).Limit(...).Find(&result)`

### Transactions
- `Transaction(ctx, func(tx Client) error { ... })`

### Error Handling
- `TranslateError(err)` - Normalize to std sentinels
- `IsRetryable(err)`, `IsTemporary(err)`, `IsCritical(err)`

## Database-Specific Behavior

### Row-Level Locking

**PostgreSQL**: Works everywhere, supports all lock modes  
**MariaDB**: Requires InnoDB + explicit transactions

Always use locking within transactions for compatibility:

```go
db.Transaction(ctx, func(tx database.Client) error {
    return tx.Query(ctx).ForUpdate().First(&user)
})
```

See `doc.go` for complete documentation.
