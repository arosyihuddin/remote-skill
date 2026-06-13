package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Shortcut struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Combo string `json:"combo"`
}

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS shortcuts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		combo TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("db init: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (d *DB) List() ([]Shortcut, error) {
	rows, err := d.conn.Query("SELECT id, name, combo FROM shortcuts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Shortcut
	for rows.Next() {
		var s Shortcut
		if err := rows.Scan(&s.ID, &s.Name, &s.Combo); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (d *DB) Add(name, combo string) (Shortcut, error) {
	res, err := d.conn.Exec("INSERT INTO shortcuts (name, combo) VALUES (?, ?)", name, combo)
	if err != nil {
		return Shortcut{}, err
	}
	id, _ := res.LastInsertId()
	return Shortcut{ID: int(id), Name: name, Combo: combo}, nil
}

func (d *DB) Delete(id int) error {
	_, err := d.conn.Exec("DELETE FROM shortcuts WHERE id = ?", id)
	return err
}

func (d *DB) Close() error {
	return d.conn.Close()
}
