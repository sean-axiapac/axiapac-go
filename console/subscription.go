package console

import (
	"time"
)

type Subscription struct {
	ID                     int        `gorm:"column:id;primaryKey;autoIncrement"`
	Key                    string     `gorm:"column:key;type:varchar(255);not null"`
	Users                  int        `gorm:"column:users;not null"`
	Employees              int        `gorm:"column:employees;not null"`
	Edition                string     `gorm:"column:edition;type:varchar(255);not null"`
	Type                   string     `gorm:"column:type;type:varchar(255);not null"`
	Domain                 string     `gorm:"column:domain;type:varchar(255);not null"`
	SyncedAt               *time.Time `gorm:"column:syncedAt"` // nullable
	ExpiredAt              time.Time  `gorm:"column:expiredAt;not null"`
	CreatedAt              time.Time  `gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"column:updatedAt;autoUpdateTime"`
	Version                int        `gorm:"column:version;not null"`
	CustomerID             *int       `gorm:"column:customerId"`                      // nullable foreign key
	IntegrityCheckRequired int8       `gorm:"column:integrityCheckRequired;not null"` // TINYINT(3)
	Deactivated            int8       `gorm:"column:deactivated;not null"`            // TINYINT(3)
	Deployment             string     `gorm:"column:deployment;type:varchar(50);not null;default:docker"`
	Environment            string     `gorm:"column:environment;type:varchar(50);not null;default:production"`
	PNG                    bool       `gorm:"column:png;not null;default:false"`

	// Relations
	Customer Customer `gorm:"foreignKey:CustomerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
