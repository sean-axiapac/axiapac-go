package core

import (
	"bytes"
	_ "embed"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// oktediLogoPNG is the Ok Tedi Mining logo (158x50 px) embedded from the OTML
// evacuation-register template, drawn in the PDF page header.
//
//go:embed assets/oktedi_logo.png
var oktediLogoPNG []byte

const (
	oktediLogoWidth  = 158.0
	oktediLogoHeight = 50.0
)

// DeviceArea maps a clock-in device_id to the physical area it sits in. Only
// employees whose clock-in device is in this map appear on the evacuation
// register, and each is tagged with its device's area; anyone who clocked in at
// an unmapped device (or not at all) is excluded. Hardcoded for now — this is
// the reference to revisit when areas become data-driven (e.g. sourced from the
// device registry or the employee Attributes `area`).
var DeviceArea = map[string]string{
	"351494370028086": "FIFO Village",
	"351494370029795": "FIFO Village",
}

// EvacuationRegisterPDF renders the Major Projects Attendance/ Evacuation
// Register for `date` as a PDF byte slice, replicating OTML's paper register
// layout: landscape A4, one section per roster panel (each starting a new page),
// employees grouped by employer with blank Sign In/Out, WBS and Away Location
// columns for manual completion.
//
// `rows` are the attendance rows for the date (from LoadAttendance, already
// scoped to the desired project). This applies the register's own rules — keep
// only employees who CLOCKED IN (RecordCount > 0) at a mapped device, tag each
// with that device's area, and drop everyone else — so the result is a muster
// snapshot of who is actually on site. `generatedAt` (Brisbane) stamps the
// footer.
//
// Returns a pure []byte so callers can stream it and, later, persist it for
// audit without touching the renderer.
func EvacuationRegisterPDF(rows []AttendanceRow, date time.Time, generatedAt time.Time) ([]byte, error) {
	// Keep only employees who clocked in at a device mapped to an area.
	registerRows := make([]AttendanceRow, 0, len(rows))
	for _, r := range rows {
		if r.RecordCount <= 0 {
			continue
		}
		if _, ok := DeviceArea[r.DeviceID]; !ok {
			continue
		}
		registerRows = append(registerRows, r)
	}

	// panel -> employer -> rows, all levels sorted for a deterministic layout.
	panels := groupSorted(registerRows, func(r AttendanceRow) string { return r.Panel })

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0) // pagination is managed manually below
	pdf.SetMargins(0, 0, 0)
	pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(oktediLogoPNG))

	pageW, pageH := pdf.GetPageSize()
	const margin = 12.0
	right := pageW - margin

	// Column geometry (mm). Sums to the full content width.
	colWidths := []float64{27, 38, 37, 16, 16, 22.5, 22.5, 22.5, 22.5, 17, 16, 16}
	colX := []float64{margin}
	for _, w := range colWidths {
		colX = append(colX, colX[len(colX)-1]+w)
	}
	tableW := colX[len(colX)-1] - margin
	const wbsFirst = 5 // first WBS column index
	const wbsLast = 8

	const rowH = 9.0
	const rowGap = 1.3
	const colHeaderH = 8.5
	footerY := pageH - 13 // rule above the footer text
	contentBottom := footerY - 3

	serif := func(style string, size float64) {
		pdf.SetFont("Times", style, size)
	}

	// Footer on every page: generation date/time (left), page number (right).
	// fpdf invokes this automatically as each page is finalised.
	pdf.SetFooterFunc(func() {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.25)
		pdf.Line(margin, footerY, right, footerY)
		serif("I", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.Text(margin+1, footerY+4.5, generatedAt.Format("Monday, 2 January 2006"))
		pdf.Text(margin+55, footerY+4.5, generatedAt.Format("3:04:05 PM"))
		label := "Page " + strconv.Itoa(pdf.PageNo())
		pdf.Text(right-pdf.GetStringWidth(label), footerY+4.5, label)
	})

	// Page furniture: rules, logo, title, red note, panel + Day/Date lines.
	// Returns the y where content may start.
	drawPageHeader := func(panelLabel string) float64 {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(1.4)
		pdf.Line(margin, 9, right, 9)

		logoW := 36.0
		logoH := logoW * oktediLogoHeight / oktediLogoWidth
		pdf.ImageOptions("logo", margin+1, 11.5, logoW, logoH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

		serif("B", 21)
		pdf.SetTextColor(0, 0, 0)
		pdf.Text(margin+44, 21, "Major Projects Attendance/ Evacuation Register")
		serif("B", 10.5)
		pdf.SetTextColor(255, 0, 0)
		pdf.Text(margin+44, 28.5, "Note:  Do Not Sign On and Sign Off at the Same Time")
		pdf.SetTextColor(0, 0, 0)

		pdf.SetLineWidth(1.1)
		pdf.Line(margin, 32, right, 32)

		// Panel identifier + hand-filled Day / Date lines.
		serif("B", 7.5)
		pdf.Text(margin+1, 37, "Emp Roster Panel")
		pdf.SetLineWidth(0.2)
		pdf.Line(margin+1, 37.8, margin+1+pdf.GetStringWidth("Emp Roster Panel"), 37.8)
		serif("B", 10.5)
		pdf.Text(margin+30, 37.5, panelLabel)

		// Day / Date prefilled from the selected register date; the underline
		// stays so the value reads as filled onto the printed line.
		serif("B", 9)
		pdf.Text(197, 37, "Day:")
		pdf.Line(208, 37.8, 240, 37.8)
		pdf.Text(210, 37, date.Format("Monday"))
		pdf.Text(243, 37, "Date:")
		pdf.Line(254, 37.8, right, 37.8)
		pdf.Text(256, 37, date.Format("02/01/2006"))

		return 45
	}

	// Employer group header: Employer/Area, Department, Area. Shaded groups get
	// a light-gray band behind the whole block (the template zebra-stripes
	// employer groups). Returns the block height.
	drawGroupHeader := func(y float64, employerLines, departmentLines []string, area string, shaded bool) float64 {
		h := 6 + math.Max(float64(len(employerLines)), float64(len(departmentLines)))*4.5
		if shaded {
			pdf.SetFillColor(238, 238, 238)
			pdf.Rect(margin, y, tableW, h+colHeaderH, "F")
		}
		// "Employer/Area" label sits on a gray chip in both shaded and unshaded groups.
		serif("B", 7.5)
		pdf.SetFillColor(225, 225, 225)
		pdf.Rect(margin+1, y+1.5, pdf.GetStringWidth("Employer/Area")+2, 4.5, "F")
		pdf.Text(margin+2, y+4.5, "Employer/Area")
		pdf.SetLineWidth(0.2)
		pdf.Line(margin+2, y+5.3, margin+2+pdf.GetStringWidth("Employer/Area"), y+5.3)

		serif("B", 10.5)
		for i, line := range employerLines {
			pdf.Text(margin+26, y+5+float64(i)*4.5, line)
		}

		serif("B", 7.5)
		pdf.Text(75, y+4.5, "Department")
		pdf.Line(75, y+5.3, 75+pdf.GetStringWidth("Department"), y+5.3)
		serif("B", 10.5)
		for i, line := range departmentLines {
			pdf.Text(92, y+5+float64(i)*4.5, line)
		}

		serif("B", 7.5)
		pdf.Text(155, y+4.5, "Area")
		pdf.Line(155, y+5.3, 155+pdf.GetStringWidth("Area"), y+5.3)
		serif("B", 10.5)
		pdf.Text(164, y+5, area)

		return h
	}

	// Column header row (once per employer group; not repeated on row
	// continuation pages, matching the template).
	drawColumnHeader := func(y float64) float64 {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.25)
		pdf.Rect(margin, y, tableW, colHeaderH, "D")
		for i := 1; i < len(colX)-1; i++ {
			pdf.Line(colX[i], y, colX[i], y+colHeaderH)
		}

		serif("", 9)
		single := []string{"Employee Code", "Employee Name", "Classification", "Sign In", "Time"}
		for i, label := range single {
			pdf.Text(colX[i]+1.5, y+4, label)
		}
		for i := wbsFirst; i <= wbsLast; i++ {
			pdf.Text(colX[i]+1.5, y+3.5, "WBS #")
			pdf.Text(colX[i]+1.5, y+7.5, "Hours Worked")
		}
		pdf.Text(colX[9]+1.5, y+3.5, "Away")
		pdf.Text(colX[9]+1.5, y+7.5, "Location")
		pdf.Text(colX[10]+1.5, y+4, "Sign Out")
		pdf.Text(colX[11]+1.5, y+4, "Time")
		return colHeaderH
	}

	// One employee row: bordered box, EMP code, name, italic classification,
	// blank fill-in cells, gray sub-divider across the WBS columns.
	drawRow := func(y float64, r AttendanceRow) {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.25)
		pdf.Rect(margin, y, tableW, rowH, "D")
		for i := 1; i < len(colX)-1; i++ {
			pdf.Line(colX[i], y, colX[i], y+rowH)
		}

		serif("", 9)
		pdf.Text(colX[0]+1.5, y+4, "EMP")
		pdf.Text(colX[0]+10, y+4, r.Code)
		pdf.Text(colX[1]+1.5, y+4, clipText(pdf, strings.TrimSpace(r.FirstName+" "+r.Surname), colWidths[1]-3))
		// Sign-in time prefilled from the kiosk clock-on record ("Sign In" itself
		// stays blank — it is the signature cell).
		if r.ClockOn != nil {
			pdf.Text(colX[4]+1.5, y+4, *r.ClockOn)
		}
		serif("I", 9)
		pdf.Text(colX[2]+1.5, y+4, clipText(pdf, r.Classification, colWidths[2]-3))

		pdf.SetDrawColor(170, 170, 170)
		pdf.SetLineWidth(0.15)
		pdf.Line(colX[wbsFirst], y+rowH/2, colX[wbsLast+1], y+rowH/2)
		pdf.SetDrawColor(0, 0, 0)
	}

	var currentPanel string
	var y float64
	newPage := func() {
		pdf.AddPage()
		y = drawPageHeader(currentPanel)
	}

	if len(panels) == 0 {
		currentPanel = "No Panel"
		newPage()
		serif("I", 11)
		pdf.SetTextColor(0, 0, 0)
		pdf.Text(margin+1, y+6, "No employees clocked in for this date.")
	}

	for _, panel := range panels {
		if panel.key == "" {
			currentPanel = "No Panel"
		} else {
			currentPanel = panel.key
		}
		newPage()

		employers := groupSorted(panel.rows, func(r AttendanceRow) string { return r.Employer })
		for groupIndex, employer := range employers {
			shaded := groupIndex%2 == 1
			first := employer.rows[0]
			serif("B", 10.5)
			name := employer.key
			if name == "" {
				name = "No Employer"
			}
			employerLines := splitLines(pdf, name, 34)
			// Department wraps within the space before the Area label (x 92–155).
			departmentLines := splitLines(pdf, first.Department, 60)
			groupHeaderH := 6 + math.Max(float64(len(employerLines)), float64(len(departmentLines)))*4.5

			// Keep the group header, column header and at least one row together.
			if y+groupHeaderH+colHeaderH+rowH+rowGap > contentBottom {
				newPage()
			}
			y += drawGroupHeader(y, employerLines, departmentLines, DeviceArea[first.DeviceID], shaded)
			y += drawColumnHeader(y)
			y += rowGap

			sorted := append([]AttendanceRow(nil), employer.rows...)
			sort.SliceStable(sorted, func(i, j int) bool { return naturalLess(sorted[i].Code, sorted[j].Code) })
			for _, r := range sorted {
				if y+rowH > contentBottom {
					newPage()
					y += 2 // template continuation pages start rows just below the header
				}
				drawRow(y, r)
				y += rowH + rowGap
			}
			y += 2.5 // spacing before the next employer group
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// group is one keyed bucket of attendance rows, preserved in sorted key order.
type group struct {
	key  string
	rows []AttendanceRow
}

// groupSorted buckets rows by key and returns the buckets ordered by key
// (natural/numeric-aware, matching the client's localeCompare({numeric:true})).
func groupSorted(rows []AttendanceRow, key func(AttendanceRow) string) []group {
	buckets := map[string][]AttendanceRow{}
	var order []string
	for _, r := range rows {
		k := key(r)
		if _, ok := buckets[k]; !ok {
			order = append(order, k)
		}
		buckets[k] = append(buckets[k], r)
	}
	sort.SliceStable(order, func(i, j int) bool { return naturalLess(order[i], order[j]) })
	out := make([]group, 0, len(order))
	for _, k := range order {
		out = append(out, group{key: k, rows: buckets[k]})
	}
	return out
}

// splitLines wraps text to a max width (mm) under the current font, returning at
// least one (possibly empty) line so callers can size a block by line count.
func splitLines(pdf *fpdf.Fpdf, text string, w float64) []string {
	if text == "" {
		return []string{""}
	}
	raw := pdf.SplitLines([]byte(text), w)
	if len(raw) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		out = append(out, string(l))
	}
	return out
}

// clipText truncates text (no ellipsis, like the template) to fit maxWidth mm
// under the current font. Trims by rune so multi-byte names aren't split.
func clipText(pdf *fpdf.Fpdf, text string, maxWidth float64) string {
	r := []rune(text)
	for len(r) > 0 && pdf.GetStringWidth(string(r)) > maxWidth {
		r = r[:len(r)-1]
	}
	return string(r)
}

// naturalLess reports a < b with digit runs compared numerically and letters
// case-insensitively — an approximation of JS localeCompare({numeric:true}) so
// codes like "EMP2" sort before "EMP10".
func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		da := a[ia] >= '0' && a[ia] <= '9'
		db := b[ib] >= '0' && b[ib] <= '9'
		if da && db {
			ja, jb := ia, ib
			for ja < len(a) && a[ja] >= '0' && a[ja] <= '9' {
				ja++
			}
			for jb < len(b) && b[jb] >= '0' && b[jb] <= '9' {
				jb++
			}
			na := strings.TrimLeft(a[ia:ja], "0")
			nb := strings.TrimLeft(b[ib:jb], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			ia, ib = ja, jb
			continue
		}
		ca, cb := toLowerASCII(a[ia]), toLowerASCII(b[ib])
		if ca != cb {
			return ca < cb
		}
		ia++
		ib++
	}
	return len(a)-ia < len(b)-ib
}

func toLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
