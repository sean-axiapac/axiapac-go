package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/infrastructure/filesystem"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BUCKET     = "axiapac-calendar"
	BUCKET_PNG = "axiapac-calendar-png"
)

type PublicHolidayRegion struct {
	Code string
	Days []models.RegionNonWorkingDay
}

type SyncStats struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

func parseExcelDate(dateStr string) (time.Time, error) {
	// Try parsing as ISO date first
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}
	// Try other common formats
	formats := []string{"01-02-06", "1/2/06", "02/01/2006", "2/1/2006", "2006/01/02", "02-Jan-2006", "2006-01-02T15:04:05Z"}
	for _, fmtStr := range formats {
		if t, err := time.Parse(fmtStr, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format: %s", dateStr)
}

func GetPublicHolidays(ctx context.Context) ([]PublicHolidayRegion, error) {
	bucket := BUCKET
	fmt.Printf("[INFO] Fetching files from bucket: %s\n", bucket)

	keys, err := filesystem.ListFiles(bucket, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var result []PublicHolidayRegion

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), ".xlsx") && !strings.HasSuffix(strings.ToLower(key), ".xls") {
			continue
		}

		// Skip files starting with underscore (hidden or temporary files)
		if strings.HasPrefix(key, "_") || strings.Contains(key, "/_") {
			continue
		}

		fmt.Printf("[INFO] Processing file: %s\n", key)
		var buf bytes.Buffer
		err := filesystem.ReadFile(bucket, key, ctx, &buf)
		if err != nil {
			fmt.Printf("[ERROR] failed to read file %s: %v\n", key, err)
			continue
		}

		f, err := excelize.OpenReader(&buf)
		if err != nil {
			fmt.Printf("[ERROR] failed to open excel file %s: %v\n", key, err)
			continue
		}
		defer f.Close()

		sheets := f.GetSheetList()
		for _, sheet := range sheets {
			rows, err := f.GetRows(sheet)
			if err != nil {
				fmt.Printf("[ERROR] failed to get rows from sheet %s in file %s: %v\n", sheet, key, err)
				continue
			}

			if len(rows) < 1 {
				continue
			}

			// Find maximum number of columns across all rows in this sheet
			maxCols := 0
			for _, row := range rows {
				if len(row) > maxCols {
					maxCols = len(row)
				}
			}

			// Row 0: Headers (Region Codes)
			headers := rows[0]
			var fileHolidays []*PublicHolidayRegion

			// Create mappings for all holiday columns (index 1 to maxCols-1)
			for i := 1; i < maxCols; i++ {
				code := ""
				if i < len(headers) {
					code = strings.ToUpper(strings.TrimSpace(headers[i]))
				}

				var region *PublicHolidayRegion
				for j := range result {
					if result[j].Code == code {
						region = &result[j]
						break
					}
				}

				if region == nil {
					result = append(result, PublicHolidayRegion{Code: code})
					region = &result[len(result)-1]
				}
				fileHolidays = append(fileHolidays, region)
			}

			// Subsequence rows: Data
			for r := 1; r < len(rows); r++ {
				row := rows[r]
				// skip empty row or row with no date
				if len(row) == 0 || row[0] == "" {
					continue
				}

				date, err := parseExcelDate(row[0])
				if err != nil {
					fmt.Printf("[WARN] could not parse date '%s' on row %d, sheet %s: %v\n", row[0], r+1, sheet, err)
					continue
				}

				for i := 1; i < len(row); i++ {
					// break when row columns exceed fileHolidays
					if i-1 >= len(fileHolidays) {
						break
					}
					dayDesc := strings.TrimSpace(row[i])
					if dayDesc != "" {
						fileHolidays[i-1].Days = append(fileHolidays[i-1].Days, models.RegionNonWorkingDay{
							Date:                    date,
							PayrollTimeTypeCategory: "PH",
							Description:             dayDesc,
							Source:                  "MASTER",
						})
					}
				}
			}
		}
	}

	return result, nil
}

func SyncHolidays(db *gorm.DB, holidayRegions []PublicHolidayRegion, dryRun bool) (SyncStats, error) {
	var toCreate []models.RegionNonWorkingDay
	var toUpdate []models.RegionNonWorkingDay
	var toDeleteIDs []int32

	if len(holidayRegions) == 0 {
		return SyncStats{}, nil
	}

	// 1. Fetch ALL calendar regions from the database
	var allRegions []models.CalendarRegion
	if err := db.Find(&allRegions).Error; err != nil {
		return SyncStats{}, fmt.Errorf("failed to fetch all regions: %w", err)
	}

	// 2. Map regions by State for easy lookup (holidayRegion.Code corresponds to State)
	regionsByState := make(map[string][]models.CalendarRegion)
	for _, r := range allRegions {
		state := strings.ToUpper(strings.TrimSpace(r.State))
		regionsByState[state] = append(regionsByState[state], r)
	}

	// 3. Fetch ALL existing holiday records in one query
	var allExistingDays []models.RegionNonWorkingDay
	if err := db.Find(&allExistingDays).Error; err != nil {
		return SyncStats{}, fmt.Errorf("failed to fetch all existing days: %w", err)
	}

	daysByRegion := make(map[int32][]models.RegionNonWorkingDay)
	for _, d := range allExistingDays {
		daysByRegion[d.CalendarRegionID] = append(daysByRegion[d.CalendarRegionID], d)
	}

	// 4. Process each S3 holiday region in memory
	for _, holidayRegion := range holidayRegions {
		code := strings.ToUpper(strings.TrimSpace(holidayRegion.Code))
		regions, ok := regionsByState[code]
		if !ok {
			fmt.Printf("[WARN] No calendar regions found with state: %s\n", code)
			continue
		}

		for _, region := range regions {
			existingDays := daysByRegion[region.CalendarRegionID]
			existingMaster := make(map[string]*models.RegionNonWorkingDay)
			for i := range existingDays {
				if existingDays[i].Source == "MASTER" {
					existingMaster[existingDays[i].Date.Format("2006-01-02")] = &existingDays[i]
				}
			}

			// loop each day in the region
			for _, hDay := range holidayRegion.Days {
				dateStr := hDay.Date.Format("2006-01-02")

				// Identify non-MASTER records to delete (they overlap with an S3 holiday)
				for i := range existingDays {
					if existingDays[i].Source != "MASTER" && existingDays[i].Date.Equal(hDay.Date) {
						toDeleteIDs = append(toDeleteIDs, existingDays[i].RegionNonWorkingDayID)
					}
				}

				if existing, ok := existingMaster[dateStr]; ok {
					// Do the update if required (only Description)
					if existing.Description != hDay.Description {
						upd := *existing
						upd.Description = hDay.Description
						toUpdate = append(toUpdate, upd)
					}
				} else {
					// Create required
					newDay := models.RegionNonWorkingDay{
						CalendarRegionID:        region.CalendarRegionID,
						Date:                    hDay.Date,
						PayrollTimeTypeCategory: hDay.PayrollTimeTypeCategory,
						Description:             hDay.Description,
						Source:                  "MASTER",
					}
					toCreate = append(toCreate, newDay)
				}
			}
		}
	}

	fmt.Printf("[INFO] Dry run (%v): %d days to create, %d days to update, %d days to delete\n", dryRun, len(toCreate), len(toUpdate), len(toDeleteIDs))

	stats := SyncStats{
		Created: len(toCreate),
		Updated: len(toUpdate),
		Deleted: len(toDeleteIDs),
	}

	if dryRun {
		return stats, nil
	}

	return stats, db.Transaction(func(tx *gorm.DB) error {
		// 1. One query to delete
		if len(toDeleteIDs) > 0 {
			uniqueIDs := make(map[int32]struct{})
			var cleanIDs []int32
			for _, id := range toDeleteIDs {
				if _, exists := uniqueIDs[id]; !exists {
					uniqueIDs[id] = struct{}{}
					cleanIDs = append(cleanIDs, id)
				}
			}
			if err := tx.Delete(&models.RegionNonWorkingDay{}, cleanIDs).Error; err != nil {
				return fmt.Errorf("failed batch delete: %w", err)
			}
		}

		// 2. One query to create
		if len(toCreate) > 0 {
			if err := tx.CreateInBatches(toCreate, 100).Error; err != nil {
				return fmt.Errorf("failed batch create: %w", err)
			}
		}

		// 3. One query to update (Update Description only)
		if len(toUpdate) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "CalendarRegionId"}, {Name: "Date"}, {Name: "Source"}},
				DoUpdates: clause.AssignmentColumns([]string{"Description"}),
			}).CreateInBatches(toUpdate, 100).Error; err != nil {
				return fmt.Errorf("failed batch update: %w", err)
			}
		}

		return nil
	})
}
