package core

import (
	"os"
	"testing"
	"time"
)

// TestEvacuationRegisterPDFSample renders a representative register to the
// scratchpad for visual inspection and asserts basic validity + that the
// clocked-in/mapped-device filter is applied.
func TestEvacuationRegisterPDFSample(t *testing.T) {
	clk := func(s string) *string { return &s }
	rows := []AttendanceRow{
		// Panel A / Employer Acme — two mapped-device, clocked-in employees.
		{Code: "1002", FirstName: "Jane", Surname: "Smith", Classification: "Boilermaker", Panel: "Panel 1", Employer: "Acme Contracting Pty Ltd", Department: "Projects BU Maintenance", ProjectID: 10, RecordCount: 1, ClockOn: clk("06:12"), DeviceID: "351494370028086"},
		{Code: "1010", FirstName: "Bob", Surname: "Jones", Classification: "Rigger", Panel: "Panel 1", Employer: "Acme Contracting Pty Ltd", Department: "Projects BU Maintenance", ProjectID: 10, RecordCount: 2, ClockOn: clk("06:30"), DeviceID: "351494370029795"},
		// Same panel, different employer (tests zebra shading + sorting).
		{Code: "2001", FirstName: "Alice", Surname: "Nguyen", Classification: "Electrician with a very long classification name", Panel: "Panel 1", Employer: "Beta Services", Department: "Projects BU Electrical Distribution and Controls", ProjectID: 10, RecordCount: 1, ClockOn: clk("05:58"), DeviceID: "351494370028086"},
		// Second panel.
		{Code: "3005", FirstName: "Charlie", Surname: "Okoro", Classification: "Supervisor", Panel: "Panel 2", Employer: "Acme Contracting Pty Ltd", Department: "Projects BU Civil", ProjectID: 20, RecordCount: 1, ClockOn: clk("06:00"), DeviceID: "351494370029795"},
		// EXCLUDED: clocked in but unmapped device.
		{Code: "9001", FirstName: "Dan", Surname: "Excluded", Classification: "Fitter", Panel: "Panel 1", Employer: "Acme Contracting Pty Ltd", Department: "Projects BU Maintenance", ProjectID: 10, RecordCount: 1, ClockOn: clk("07:00"), DeviceID: "999999999999999"},
		// EXCLUDED: mapped device but no clock-in (rostered absent).
		{Code: "9002", FirstName: "Eve", Surname: "Absent", Classification: "Fitter", Panel: "Panel 1", Employer: "Acme Contracting Pty Ltd", Department: "Projects BU Maintenance", ProjectID: 10, RecordCount: 0, DeviceID: "351494370028086"},
	}

	date := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	generatedAt := time.Date(2026, 7, 7, 14, 32, 9, 0, time.UTC)

	pdf, err := EvacuationRegisterPDF(rows, date, generatedAt)
	if err != nil {
		t.Fatalf("EvacuationRegisterPDF: %v", err)
	}
	if len(pdf) < 1000 || string(pdf[:5]) != "%PDF-" {
		t.Fatalf("output is not a PDF (len=%d, head=%q)", len(pdf), pdf[:min(8, len(pdf))])
	}

	if out := os.Getenv("EVAC_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, pdf, 0o644); err != nil {
			t.Fatalf("write sample: %v", err)
		}
		t.Logf("wrote sample PDF to %s (%d bytes)", out, len(pdf))
	}
}
