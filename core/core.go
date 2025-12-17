package core

import (
	"axiapac.com/axiapac/core/models"
	"gorm.io/gorm"
)

type ProjectCostCentre struct {
	models.JobCostCentre
	Code        string
	FullCode    string
	Description string
	ParentID    int32
}

func GetJobCostCentresWithCostCentre(db *gorm.DB) ([]ProjectCostCentre, error) {
	var results []ProjectCostCentre

	err := db.Model(&models.JobCostCentre{}).
		Joins("JOIN costcentres ON costcentres.costcentreid = jobcostcentres.costcentreid").
		Select(`jobcostcentres.*, 
                costcentres.code AS code, 
                costcentres.fullcode AS full_code, 
                costcentres.description AS description,
                costcentres.parentid AS parent_id`).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}
