// Package db opens GORM connections for Sheidan apps.
//
// A DSN names either a local SQLite file (file: scheme, served by the
// mattn/go-sqlite3 driver) or a remote Turso or sqld database
// (libsql://, http://, https://, ws://, wss:// scheme, served by the
// tursodatabase/libsql-client-go driver).
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	sqlite "codeberg.org/morelj/gorm-sqlite-libsql"
	// Registers "sqlite3", the driver the libsql driver delegates
	// file: DSNs to.
	_ "github.com/mattn/go-sqlite3"
	"github.com/tursodatabase/libsql-client-go/libsql"
	"gorm.io/gorm"
)

// Option configures Open.
type Option func(*config)

// config holds the applied Open options.
type config struct {
	authToken string
}

// WithAuthToken sets the auth token for a remote connection. The
// libsql driver rejects auth tokens in the DSN, so the token is
// applied to the connector instead. Local file: connections ignore
// it.
func WithAuthToken(token string) Option {
	return func(c *config) {
		c.authToken = token
	}
}

// Open opens a GORM connection for the given DSN.
//
// A file: DSN opens a local SQLite file, creating its parent
// directory when needed. A libsql://, http://, https://, ws://, or
// wss:// DSN opens a remote Turso or sqld database.
func Open(dsn string, opts ...Option) (*gorm.DB, error) {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := prepareFileDir(dsn); err != nil {
		return nil, err
	}
	dialector, err := dialectorFor(dsn, cfg)
	if err != nil {
		return nil, err
	}
	return gorm.Open(dialector, &gorm.Config{})
}

// dialectorFor selects the GORM dialector for a DSN. A DSN with an
// auth token uses a libsql connector, because the libsql driver
// rejects auth tokens in the DSN.
func dialectorFor(dsn string, cfg config) (gorm.Dialector, error) {
	if cfg.authToken != "" {
		connector, err := libsql.NewConnector(dsn, libsql.WithAuthToken(cfg.authToken))
		if err != nil {
			return nil, err
		}
		return sqlite.New(sqlite.Config{Conn: sql.OpenDB(connector)}), nil
	}
	return sqlite.Open(dsn), nil
}

// prepareFileDir creates the parent directory of a file: DSN, so the
// mattn driver can create the database file. Other DSNs are ignored.
func prepareFileDir(dsn string) error {
	if !strings.HasPrefix(dsn, "file:") {
		return nil
	}
	path := strings.TrimPrefix(dsn, "file:")
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" || path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
