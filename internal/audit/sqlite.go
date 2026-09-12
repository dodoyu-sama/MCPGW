package audit

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS calls (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id   TEXT    NOT NULL,
		tool_name   TEXT    NOT NULL,
		params      TEXT,
		raw_params  TEXT,
		raw_result  TEXT,
		result      TEXT,
		error_msg   TEXT,
		latency_ms  INTEGER,
		timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_calls_client ON calls(client_id);
	CREATE INDEX IF NOT EXISTS idx_calls_tool   ON calls(tool_name);
	CREATE INDEX IF NOT EXISTS idx_calls_time   ON calls(timestamp);`); err != nil {
		return err
	}
	// Best-effort schema evolution for existing databases (column may already exist).
	for _, ddl := range []string{
		`ALTER TABLE calls ADD COLUMN raw_params TEXT`,
		`ALTER TABLE calls ADD COLUMN raw_result TEXT`,
	} {
		_, _ = s.db.Exec(ddl)
	}
	return nil
}

func (s *SQLiteStore) Insert(r *CallRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO calls (client_id, tool_name, params, raw_params, raw_result, result, error_msg, latency_ms, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ClientID, r.ToolName, r.Params, r.RawParams, r.RawResult, r.Result, r.ErrorMsg, r.LatencyMs, r.Timestamp,
	)
	return err
}

func (s *SQLiteStore) Query(opts QueryOpts) ([]CallRecord, error) {
	query := `SELECT id, client_id, tool_name, params, raw_params, raw_result, result, error_msg, latency_ms, timestamp
	          FROM calls WHERE 1=1`
	var args []interface{}
	if opts.ClientID != "" {
		query += " AND client_id = ?"
		args = append(args, opts.ClientID)
	}
	if opts.ToolName != "" {
		query += " AND tool_name = ?"
		args = append(args, opts.ToolName)
	}
	if !opts.Since.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}
	if !opts.Until.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, opts.Until)
	}
	query += " ORDER BY id DESC"
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CallRecord, 0)
	for rows.Next() {
		var r CallRecord
		if err := rows.Scan(&r.ID, &r.ClientID, &r.ToolName, &r.Params, &r.RawParams, &r.RawResult,
			&r.Result, &r.ErrorMsg, &r.LatencyMs, &r.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Get(id int64) (*CallRecord, error) {
	var r CallRecord
	err := s.db.QueryRow(
		`SELECT id, client_id, tool_name, params, raw_params, raw_result, result, error_msg, latency_ms, timestamp
		 FROM calls WHERE id = ?`, id,
	).Scan(&r.ID, &r.ClientID, &r.ToolName, &r.Params, &r.RawParams, &r.RawResult,
		&r.Result, &r.ErrorMsg, &r.LatencyMs, &r.Timestamp)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *SQLiteStore) Stats(opts StatsOpts) (*Stats, error) {
	stats := &Stats{ToolCounts: map[string]int64{}}
	query := `SELECT COUNT(*),
	                 COALESCE(SUM(CASE WHEN error_msg != '' THEN 1 ELSE 0 END), 0),
	                 COALESCE(AVG(latency_ms), 0)
	          FROM calls WHERE 1=1`
	var args []interface{}
	if !opts.Since.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}
	if !opts.Until.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, opts.Until)
	}
	if err := s.db.QueryRow(query, args...).Scan(&stats.TotalCalls, &stats.ErrorCount, &stats.AvgLatencyMs); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT tool_name, COUNT(*) FROM calls GROUP BY tool_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var cnt int64
		if err := rows.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		stats.ToolCounts[name] = cnt
	}
	return stats, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
