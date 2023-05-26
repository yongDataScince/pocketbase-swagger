package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type ModelCU struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ID
}

type ModelCUPure struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ModelC struct {
	CreatedAt time.Time `json:"created_at"`
	ID
}

type ID struct {
	ID uuid.UUID `json:"id" gorm:"primarykey,type:uuid" example:"cf8a07d4-077e-402e-a46b-ac0ed50989ec"`
}

type Groups struct {
	Groups datatypes.JSON `json:"groups" example:"admin,group1" swaggertype:"array,string"`
}

type UserData struct {
	Name  string `json:"name" gorm:"unique;uniqueIndex;not null" example:"userX"`
	Email string `json:"email" example:"userx@worldline.com"`
	Groups
}

type UserPrivate struct {
	Password string `json:"password" gorm:"not null" example:"pass1234"`
}

type UserPure struct {
	UserPrivate
	UserData
}

type User struct {
	UserPure
	ModelCU
}
