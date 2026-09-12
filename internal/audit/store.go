package audit

import "log"

// NewStore builds an audit store from the driver name and DSN.
// Supported drivers: "sqlite" (default) and "postgres".
func NewStore(driver, dsn string) (Store, error) {
	switch driver {
	case "sqlite", "":
		return NewSQLite(dsn)
	case "postgres":
		return NewPostgres(dsn)
	default:
		log.Printf("warn: unknown audit driver %q, falling back to sqlite", driver)
		return NewSQLite(dsn)
	}
}
