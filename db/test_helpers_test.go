package db_test

import (
	"context"
	"os"

	. "github.com/onsi/gomega"

	golib "github.com/nojyerac/go-lib/db"
)

const (
	migrationsPath = "../migrations"
	upSQLSuffix    = ".up.sql"
)

// setupTestDatabase creates an in-memory SQLite database and runs migrations
func setupTestDatabase(ctx context.Context) golib.Database {
	// Create in-memory SQLite database
	config := &golib.Configuration{
		Driver:    "sqlite",
		DBConnStr: ":memory:",
	}
	database := golib.NewDatabase(config)

	// Open the database connection
	err := database.Open(ctx)
	Expect(err).NotTo(HaveOccurred())

	// Enable foreign key constraints for SQLite
	_, err = database.Exec(ctx, "PRAGMA foreign_keys = ON;")
	Expect(err).NotTo(HaveOccurred())

	// Run migrations
	files, err := os.ReadDir(migrationsPath)
	Expect(err).NotTo(HaveOccurred())

	for _, file := range files {
		if file.IsDir() || len(file.Name()) < 7 || file.Name()[len(file.Name())-7:] != upSQLSuffix {
			continue
		}
		migration, err := os.ReadFile(migrationsPath + "/" + file.Name())
		Expect(err).NotTo(HaveOccurred())

		_, err = database.Exec(ctx, string(migration))
		Expect(err).NotTo(HaveOccurred())
	}

	return database
}
