-- Add the `overtime` column to oktedi_timesheets.
-- Mirrors model.OktediTimesheet.Overtime (oktedi/model/timesheet.go):
--   Overtime float64 `gorm:"column:overtime;type:decimal(10,2);not null"`
--
-- NOT NULL with DEFAULT 0.00 so existing rows backfill cleanly.
-- MySQL/MariaDB.

ALTER TABLE `oktedi_timesheets`
    ADD COLUMN `overtime` DECIMAL(10,2) NOT NULL DEFAULT 0.00 AFTER `break`;

-- Rollback:
-- ALTER TABLE `oktedi_timesheets` DROP COLUMN `overtime`;
