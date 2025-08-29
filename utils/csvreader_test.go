package utils

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	csvData := `name,age,city
Alice,30,New York
Bob,25,Los Angeles`

	reader := strings.NewReader(csvData)

	got, err := ParseCSV(reader)
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	want := [][]string{
		{"name", "age", "city"},
		{"Alice", "30", "New York"},
		{"Bob", "25", "Los Angeles"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseCSV returned %+v, want %+v", got, want)
	}
}
