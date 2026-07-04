package core

import (
	"encoding/json"
	"strconv"

	"axiapac.com/axiapac/core/models"
)

// ParseAttributes parses an employee's Attributes JSON into a map. Returns nil
// when Attributes is empty or invalid, so lookups on the result simply miss.
func ParseAttributes(emp models.Employee) map[string]any {
	if emp.Attributes == "" {
		return nil
	}
	var attrs map[string]any
	if json.Unmarshal([]byte(emp.Attributes), &attrs) != nil {
		return nil
	}
	return attrs
}

// AttrString returns attrs[key] as a string. Numeric values are formatted;
// absent, null or other-typed values return "".
func AttrString(attrs map[string]any, key string) string {
	switch v := attrs[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// AttrRefID returns the id of a reference-shaped attribute value {"id": N}
// (the shape used by employer, spo, manager, backToBack). Returns 0 when the
// key is absent or not reference-shaped.
func AttrRefID(attrs map[string]any, key string) int32 {
	ref, ok := attrs[key].(map[string]any)
	if !ok {
		return 0
	}
	id, ok := ref["id"].(float64)
	if !ok {
		return 0
	}
	return int32(id)
}
