package core

import (
	"errors"

	"axiapac.com/axiapac/core/models"
	"gorm.io/gorm"
)

type Employee struct {
	models.Employee
	LabourCode *string
	LabourCost *float64
}

func FindEmployeeByID(db *gorm.DB, id int) (*models.Employee, error) {
	var emp models.Employee
	result := db.First(&emp, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil // not found
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &emp, nil
}

func GetEmployeesWithLabourRates(db *gorm.DB) ([]Employee, error) {
	var employees []Employee

	err := db.Model(&Employee{}).
		Select("employees.*, labourrates.Code as labour_code, labourrates.Cost as labour_cost").
		Joins("LEFT OUTER JOIN labourrates ON labourrates.labourrateid = employees.labourrateid").
		Scan(&employees).Error

	if err != nil {
		return nil, err
	}

	return employees, nil
}

func FindEmployeeOnCost(db *gorm.DB, employeeID int32, payrollTimeTypeID int32) (*models.EmployeeOnCost, error) {
	var oncost *models.EmployeeOnCost
	if err := db.Model(&models.EmployeeOnCost{}).Where(&models.EmployeeOnCost{EmployeeID: employeeID, PayrollTimeTypeID: payrollTimeTypeID}).
		First(&oncost).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not found
		}
		return nil, err
	}
	return oncost, nil
}

func FindEmployeeCostOfEmployement(db *gorm.DB, employeeID int32) (*models.EmployeesCostofEmployment, error) {
	var empCost *models.EmployeesCostofEmployment
	if err := db.Model(&models.EmployeesCostofEmployment{}).Where(&models.EmployeesCostofEmployment{EmployeesCostofEmploymentID: employeeID}).
		First(&empCost).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not found
		}
		return nil, err
	}
	return empCost, nil
}

func CalcEmployeeRate(db *gorm.DB, employee *models.Employee, labourRate *models.LabourRate, timeType *models.PayrollTimeType) (float64, error) {
	// non-working labour rate means no cost
	if labourRate != nil && labourRate.NonWorking {
		return 0, nil
	}

	// check employee oncost for this payroll time type
	oncost, err := FindEmployeeOnCost(db, employee.EmployeeID, timeType.PayrollTimeTypeID)
	if err != nil {
		return 0, err
	}
	if oncost != nil {
		return oncost.Rate, nil
	}

	// fallback to employee cost of employment
	empCost, err := FindEmployeeCostOfEmployement(db, employee.EmployeeID)
	if err != nil {
		return 0, err
	}
	if empCost != nil {
		return empCost.ActualLabourCostRate, nil
	}

	// fallback to labour rate
	if labourRate != nil {
		return labourRate.Cost, nil
	}

	return 0, errors.New("no rate found for employee " + employee.Code)
}
