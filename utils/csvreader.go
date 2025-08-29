package utils

import (
	"encoding/csv"
	"io"
)

func ParseCSV(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}
