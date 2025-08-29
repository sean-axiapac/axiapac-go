package console

import (
	"time"
)

type Customer struct {
	ID        int       `gorm:"primaryKey;autoIncrement;column:id"`
	Code      string    `gorm:"size:255;not null;unique;column:code"`
	Name      string    `gorm:"size:255;not null;unique;column:name"`
	Email     string    `gorm:"size:255;not null;unique;column:email"`
	ABN       *string   `gorm:"size:255;unique;column:abn"`
	CreatedAt time.Time `gorm:"precision:6;autoCreateTime;column:createdAt"`
	UpdatedAt time.Time `gorm:"precision:6;autoUpdateTime;column:updatedAt"`
	Version   int       `gorm:"not null;column:version"`
}
