package helper

import (
	"strings"
	"testing"
)

func TestParseClockInCSV(t *testing.T) {
	csvData := `ID,UserID,Timestamp,Location
1,user1,2023-08-20T09:00:00+00:00,Office
2,user2,2023-08-20T10:00:00+00:00,Remote
`
	records, err := ParseClockInCSV(strings.NewReader(csvData), 10*60*60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ID != 1 || records[0].UserID != "user1" || records[0].Location != "Office" || records[0].Date != "2023-08-20" {
		t.Errorf("unexpected first record: %+v", records[0])
	}

	if records[1].ID != 2 || records[1].UserID != "user2" || records[1].Location != "Remote" || records[1].Date != "2023-08-20" {
		t.Errorf("unexpected second record: %+v", records[1])
	}
}
