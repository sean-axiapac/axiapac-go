package console

import (
	"errors"

	"gorm.io/gorm"
)

func GetCustomers(db *gorm.DB) ([]Customer, error) {
	var customers []Customer
	err := db.Find(&customers).Error
	return customers, err
}

func FindSubscriptionByDomain(db *gorm.DB, domain string) (*Subscription, error) {
	var sub Subscription
	err := db.Where(&Subscription{Domain: domain}).Preload("Customer").First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // not found
	}
	return &sub, err
}
