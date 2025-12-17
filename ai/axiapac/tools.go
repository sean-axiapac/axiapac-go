package axiapac

import (
	"encoding/json"

	"gorm.io/gorm"
)

// ExecuteSQL runs a raw SQL query and returns the result as a JSON string
func ExecuteSQL(db *gorm.DB, query string) (string, error) {
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		// Prepare a slice of interfaces to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into valuePtrs
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}

		// Convert into map[string]interface{}
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			rowMap[col] = v
		}
		results = append(results, rowMap)
	}

	// Marshal results to JSON string
	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
