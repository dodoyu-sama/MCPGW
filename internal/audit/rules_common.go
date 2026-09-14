package audit

import (
	"database/sql"
	"encoding/json"
)

// Rules are stored in the same database as call records, so SQLite keeps a
// single file lock and Postgres a single connection pool. The list-valued parts
// of a rule (patterns, fields) are stored as JSON text.

func encodeStrings(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeStrings(s string) []string {
	if s == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

const ruleColumns = `id, name, COALESCE(patterns, '[]'), COALESCE(fields, '[]'),
	COALESCE(mask_char, ''), enabled, COALESCE(source, 'ui'), created_at, updated_at`

// scanRules is shared by both backends: they only differ in placeholder syntax.
func scanRules(rows *sql.Rows) ([]MaskRule, error) {
	defer rows.Close()
	out := make([]MaskRule, 0)
	for rows.Next() {
		var r MaskRule
		var patterns, fields string
		if err := rows.Scan(&r.ID, &r.Name, &patterns, &fields, &r.MaskChar,
			&r.Enabled, &r.Source, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Patterns = decodeStrings(patterns)
		r.Fields = decodeStrings(fields)
		out = append(out, r)
	}
	return out, rows.Err()
}
