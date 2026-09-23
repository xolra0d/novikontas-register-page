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
	db, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		logger.Error("Failed to start connection to database.", "error", err.Error())
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

func (d *Database) GetCourses(ctx context.Context) ([]Course, error) {
	query := "SELECT id, name, image, locations, price FROM courses WHERE active=TRUE ORDER BY id;"
	rows, err := d.db.Query(ctx, query)
	if err != nil {
		d.logger.Error("failed to load available languages", "error", err)
		return nil, err
	}
	defer rows.Close()
	courses := []Course{}
	for rows.Next() {
		c := Course{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.Locations, &c.Price); err != nil {
			d.logger.Error("failed to load vocabulary", "error", err)
			return nil, err
		}
		courses = append(courses, c)
	}
	return courses, nil
}

func (d *Database) GetCourse(ctx context.Context, id int64) (Course, error) {
	query := "SELECT name, description, image, locations, price, active FROM courses WHERE id = $1"

	c := Course{ID: id}
	err := d.db.QueryRow(ctx, query, id).Scan(&c.Name, &c.Description, &c.Image, &c.Locations, &c.Price, &c.Available)
	if err != nil {
		d.logger.Error("failed to load available languages", "error", err)
		return Course{}, err
	}
	return c, nil
}
