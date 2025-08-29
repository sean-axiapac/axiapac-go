package helper

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"axiapac.com/axiapac/utils"
)

type Record struct {
	ID        int
	UserID    string
	Timestamp time.Time
	Date      string
	Location  string
}

type ClockRecord struct {
	UserID  string
	Date    string
	From    time.Time
	To      time.Time
	Records []Record
}

func ParseClockInCSV(r io.Reader, offset int) ([]Record, error) {
	rows, err := utils.ParseCSV(r)
	if err != nil {
		return nil, err
	}

	loc := time.FixedZone("OFFSET", offset)

	var records []Record
	for i, row := range rows {
		if i == 0 {
			continue
		}

		if len(row) < 4 {
			return nil, fmt.Errorf("row %d: expected 4 columns, got %d", i, len(row))
		}

		id, err := strconv.Atoi(row[0])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid ID: %w", i, err)
		}

		timestamp, err := time.Parse(time.RFC3339, row[2])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid timestamp: %w", i, err)
		}
		timestamp = timestamp.In(loc)

		record := Record{
			ID:        id,
			UserID:    row[1],
			Timestamp: timestamp,
			Date:      timestamp.Format("2006-01-02"),
			Location:  row[3],
		}

		records = append(records, record)
	}

	return records, nil
}

func GroupRecords(records []Record) []ClockRecord {
	grouped := make(map[string]ClockRecord)

	for _, r := range records {
		key := r.UserID + "|" + r.Date
		cr, exists := grouped[key]

		if !exists {
			// first record for this user/date
			grouped[key] = ClockRecord{
				UserID:  r.UserID,
				Date:    r.Date,
				From:    r.Timestamp,
				To:      r.Timestamp,
				Records: []Record{r},
			}
		} else {
			// update From/To and append record
			if r.Timestamp.Before(cr.From) {
				cr.From = r.Timestamp
			}
			if r.Timestamp.After(cr.To) {
				cr.To = r.Timestamp
			}
			cr.Records = append(cr.Records, r)
			grouped[key] = cr
		}
	}

	// flatten map into slice
	var clockRecords []ClockRecord
	for _, cr := range grouped {
		clockRecords = append(clockRecords, cr)
	}

	return clockRecords
}
