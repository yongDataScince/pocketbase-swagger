package registry

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Registry struct {
	DB *gorm.DB
}

var reg *Registry

func Get(connectionString string) (*Registry, error) {
	if reg == nil {
		db, err := gorm.Open(mysql.Open(connectionString), &gorm.Config{})
		if err != nil {
			return nil, err
		}

		reg = &Registry{
			DB: db,
		}
	}

	return reg, nil
}
