package core

import (
	"sort"

	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
)

type RecordGroup struct {
	Tag     string
	Date    string
	Records []*model.ClockinRecord
}

func (rg *RecordGroup) GetClockIn() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[0].Timestamp
}

func (rg *RecordGroup) GetClockOut() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[len(rg.Records)-1].Timestamp
}

// GetDeviceID returns the device the clock-in (earliest) record came from, ""
// when the group is empty. Used by the evacuation register to resolve the area
// an employee signed in at.
func (rg *RecordGroup) GetDeviceID() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[0].DeviceID
}

func GroupRecords(records []*model.ClockinRecord) []*RecordGroup {
	// group by date - although we are processing single date, the util is generic
	var groups []*RecordGroup
	dategroups := utils.GroupBy(records, func(r *model.ClockinRecord) string { return r.Date })

	for date, recs := range dategroups {
		// group by tag
		taggroups := utils.GroupBy(recs, func(r *model.ClockinRecord) string { return r.Tag })
		for tag, r2 := range taggroups {
			// Sort records by timestamp to ensure First and Last are correct
			sort.Slice(r2, func(i, j int) bool {
				return r2[i].Timestamp < r2[j].Timestamp
			})

			rg := &RecordGroup{
				Tag:     tag,
				Date:    date,
				Records: r2,
			}
			groups = append(groups, rg)
		}
	}
	return groups
}
