// Package gormlite is the SQLite backend of gormdb: a gorm client over a database file, opened
// through the pure-Go modernc driver so a binary using it builds with CGO_ENABLED=0.
//
//	db, err := gormlite.NewClient("/var/lib/app/app.db", m)
//
// Every client opens its file in WAL mode with synchronous=FULL, foreign keys on and a busy
// timeout, which is what a single-process application that must not lose a committed write wants.
package gormlite
