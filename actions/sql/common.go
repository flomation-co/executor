package sql_common

import (
	"database/sql"
	"fmt"
	"strings"
)

// OpenConnection opens a database connection using the provided driver and DSN,
// and verifies connectivity with a ping.
func OpenConnection(driverName, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// ExecuteQuery executes a SQL query against the provided database connection.
// SELECT queries return rows as a slice of maps; DML queries return the affected row count.
func ExecuteQuery(db *sql.DB, query string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "WITH") {
		return executeSelect(db, query)
	}

	return executeDML(db, query)
}

func executeSelect(db *sql.DB, query string) (map[string]interface{}, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve columns: %w", err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			val := values[i]
			// Convert []byte to string for JSON compatibility
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Ensure empty result set returns an empty slice, not nil
	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"results":   results,
		"row_count": len(results),
	}, nil
}

func executeDML(db *sql.DB, query string) (map[string]interface{}, error) {
	result, err := db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	return map[string]interface{}{
		"results":   nil,
		"row_count": rowsAffected,
	}, nil
}
