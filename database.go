package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewDatabase creates new instance of postgres client.
func NewDatabase(postgresURL string, logger *slog.Logger) *Database {
	op := "database.NewDatabase"

	db, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		logger.Error("Failed to start connection to database.", "error", err, "op", op)
		os.Exit(1)
	}
	return &Database{
		db:     db,
		logger: logger,
	}
}

// Close closes postgres pool.
func (d *Database) Close() {
	d.db.Close()
}

// GetCourses returns list of active courses. Without description.
func (d *Database) GetCourses(ctx context.Context) ([]Course, error) {
	op := "database.GetCourses"

	query := "SELECT id, name, image, locations, price FROM courses WHERE active=TRUE ORDER BY id;"
	rows, err := d.db.Query(ctx, query)
	if err != nil {
		d.logger.Error("failed to load available languages", "error", err, "op", op)
		return nil, err
	}

	defer rows.Close()
	courses := []Course{}

	for rows.Next() {
		c := Course{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.Locations, &c.Price); err != nil {
			d.logger.Error("failed to load vocabulary", "error", err, "op", op)
			return nil, err
		}
		courses = append(courses, c)
	}
	return courses, nil
}

// GetCourse retrieves single course informatiojn including description.
func (d *Database) GetCourse(ctx context.Context, id int64) (Course, error) {
	op := "database.GetCourse"

	query := "SELECT name, description, image, locations, price, active FROM courses WHERE id = $1"

	c := Course{ID: id}
	err := d.db.QueryRow(ctx, query, id).Scan(&c.Name, &c.Description, &c.Image, &c.Locations, &c.Price, &c.Available)
	if err != nil {
		d.logger.Error("failed to load available languages", "error", err, "op", op)
		return Course{}, err
	}
	return c, nil
}
