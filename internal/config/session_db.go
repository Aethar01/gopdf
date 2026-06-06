package config

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DocumentSession struct {
	Page            int
	ScrollX         float64
	ScrollY         float64
	AnchorPage      int
	AnchorX         float64
	AnchorY         float64
	AnchorValid     bool
	Zoom            float64
	FitMode         string
	RenderMode      string
	Rotation        float64
	DualPage        bool
	FirstPageOffset bool
	StatusBarShown  bool
	AltColors       bool
}

type DocumentMark struct {
	Page        int
	ScrollX     float64
	ScrollY     float64
	AnchorPage  int
	AnchorX     float64
	AnchorY     float64
	AnchorValid bool
}

func SessionDatabasePath() string {
	dir := StateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "session.sqlite")
}

func GetDocumentSession(path string) (DocumentSession, bool) {
	path = AbsoluteDocumentPath(path)
	if path == "" {
		return DocumentSession{}, false
	}
	db, err := openSessionDatabase()
	if err != nil || db == nil {
		return DocumentSession{}, false
	}
	defer db.Close()
	var s DocumentSession
	err = db.QueryRow(`
		SELECT page, scroll_x, scroll_y, anchor_page, anchor_x, anchor_y, anchor_valid,
		       zoom, fit_mode, render_mode, rotation,
		       dual_page, first_page_offset, status_bar_shown, alt_colors
		FROM document_sessions
		WHERE path = ?
	`, path).Scan(
		&s.Page,
		&s.ScrollX,
		&s.ScrollY,
		&s.AnchorPage,
		&s.AnchorX,
		&s.AnchorY,
		&s.AnchorValid,
		&s.Zoom,
		&s.FitMode,
		&s.RenderMode,
		&s.Rotation,
		&s.DualPage,
		&s.FirstPageOffset,
		&s.StatusBarShown,
		&s.AltColors,
	)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return DocumentSession{}, false
	}
	return s, true
}

func SetDocumentSession(path string, s DocumentSession) error {
	path = AbsoluteDocumentPath(path)
	if path == "" {
		return nil
	}
	db, err := openSessionDatabase()
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO document_sessions (
			path, page, scroll_x, scroll_y, anchor_page, anchor_x, anchor_y, anchor_valid,
			zoom, fit_mode, render_mode, rotation,
			dual_page, first_page_offset, status_bar_shown, alt_colors, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		ON CONFLICT(path) DO UPDATE SET
			page = excluded.page,
			scroll_x = excluded.scroll_x,
			scroll_y = excluded.scroll_y,
			anchor_page = excluded.anchor_page,
			anchor_x = excluded.anchor_x,
			anchor_y = excluded.anchor_y,
			anchor_valid = excluded.anchor_valid,
			zoom = excluded.zoom,
			fit_mode = excluded.fit_mode,
			render_mode = excluded.render_mode,
			rotation = excluded.rotation,
			dual_page = excluded.dual_page,
			first_page_offset = excluded.first_page_offset,
			status_bar_shown = excluded.status_bar_shown,
			alt_colors = excluded.alt_colors,
			updated_at = excluded.updated_at
	`, path, s.Page, s.ScrollX, s.ScrollY, s.AnchorPage, s.AnchorX, s.AnchorY, s.AnchorValid, s.Zoom, s.FitMode, s.RenderMode, s.Rotation, s.DualPage, s.FirstPageOffset, s.StatusBarShown, s.AltColors)
	return err
}

func RecordRecentFile(path string, maxEntries int) error {
	if maxEntries < 1 {
		return nil
	}
	path = AbsoluteDocumentPath(path)
	if path == "" {
		return nil
	}
	db, err := openSessionDatabase()
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	defer db.Close()
	if _, err = db.Exec(`
		INSERT INTO recent_files (path, updated_at)
		VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET updated_at = excluded.updated_at
	`, path, time.Now().UnixNano()); err != nil {
		return err
	}
	_, err = db.Exec(`
		DELETE FROM recent_files
		WHERE path NOT IN (
			SELECT path
			FROM recent_files
			ORDER BY updated_at DESC, path ASC
			LIMIT ?
		)
	`, maxEntries)
	return err
}

func RecentFiles(limit int) []string {
	if limit < 1 {
		return nil
	}
	db, err := openSessionDatabase()
	if err != nil || db == nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT path
		FROM recent_files
		ORDER BY updated_at DESC, path ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func SetDocumentMark(path string, name string, mark DocumentMark) error {
	path = AbsoluteDocumentPath(path)
	if path == "" || name == "" {
		return nil
	}
	db, err := openSessionDatabase()
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO document_marks (
			path, name, page, scroll_x, scroll_y, anchor_page, anchor_x, anchor_y, anchor_valid, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path, name) DO UPDATE SET
			page = excluded.page,
			scroll_x = excluded.scroll_x,
			scroll_y = excluded.scroll_y,
			anchor_page = excluded.anchor_page,
			anchor_x = excluded.anchor_x,
			anchor_y = excluded.anchor_y,
			anchor_valid = excluded.anchor_valid,
			updated_at = excluded.updated_at
	`, path, name, mark.Page, mark.ScrollX, mark.ScrollY, mark.AnchorPage, mark.AnchorX, mark.AnchorY, mark.AnchorValid, time.Now().UnixNano())
	return err
}

func GetDocumentMark(path string, name string) (DocumentMark, bool) {
	path = AbsoluteDocumentPath(path)
	if path == "" || name == "" {
		return DocumentMark{}, false
	}
	db, err := openSessionDatabase()
	if err != nil || db == nil {
		return DocumentMark{}, false
	}
	defer db.Close()
	var mark DocumentMark
	err = db.QueryRow(`
		SELECT page, scroll_x, scroll_y, anchor_page, anchor_x, anchor_y, anchor_valid
		FROM document_marks
		WHERE path = ? AND name = ?
	`, path, name).Scan(&mark.Page, &mark.ScrollX, &mark.ScrollY, &mark.AnchorPage, &mark.AnchorX, &mark.AnchorY, &mark.AnchorValid)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return DocumentMark{}, false
	}
	return mark, true
}

func openSessionDatabase() (*sql.DB, error) {
	path := SessionDatabasePath()
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := initSessionDatabase(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initSessionDatabase(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return err
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS document_sessions (
			path TEXT PRIMARY KEY,
			page INTEGER NOT NULL,
			scroll_x REAL NOT NULL,
			scroll_y REAL NOT NULL,
			anchor_page INTEGER NOT NULL DEFAULT 0,
			anchor_x REAL NOT NULL DEFAULT 0,
			anchor_y REAL NOT NULL DEFAULT 0,
			anchor_valid INTEGER NOT NULL DEFAULT 0,
			zoom REAL NOT NULL,
			fit_mode TEXT NOT NULL,
			render_mode TEXT NOT NULL,
			rotation REAL NOT NULL,
			dual_page INTEGER NOT NULL,
			first_page_offset INTEGER NOT NULL,
			status_bar_shown INTEGER NOT NULL,
			alt_colors INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS recent_files (
			path TEXT PRIMARY KEY,
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS document_marks (
			path TEXT NOT NULL,
			name TEXT NOT NULL,
			page INTEGER NOT NULL,
			scroll_x REAL NOT NULL,
			scroll_y REAL NOT NULL,
			anchor_page INTEGER NOT NULL,
			anchor_x REAL NOT NULL,
			anchor_y REAL NOT NULL,
			anchor_valid INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (path, name)
		)
	`); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE document_sessions ADD COLUMN anchor_page INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE document_sessions ADD COLUMN anchor_x REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE document_sessions ADD COLUMN anchor_y REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE document_sessions ADD COLUMN anchor_valid INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}
