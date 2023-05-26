package apis

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/registry"

	"golang.org/x/crypto/bcrypt"

	"gorm.io/gorm"
)

func HashPassword(password []byte) ([]byte, error) {
	// zero cost use default
	bytes, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)

	return bytes, err
}

type ModelCU struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ID
}

type Error struct {
	Error interface{} `json:"error,omitempty" example:"some problem" swaggertype:"string"`
}

type ID struct {
	ID uuid.UUID `json:"id" gorm:"primarykey,type:uuid" example:"cf8a07d4-077e-402e-a46b-ac0ed50989ec"`
}

type Meta struct {
	Limit  int    `json:"limit" query:"limit" example:"20"`
	Offset int    `json:"offset" query:"offset" example:"0"`
	Count  int64  `json:"count" query:"count" example:"35"`
	Search string `json:"search,omitempty" query:"search"`
}

type UserDataID struct {
	models.UserData
	ID
}

type UserPureID struct {
	models.UserPure
	ID
}

type UserMetaAdmin struct {
	Admin bool `json:"admin,omitempty" query:"admin"`
	Meta
}

type Data struct {
	Data interface{} `json:"data,omitempty" swaggertype:"object,string"`
}

type DataMeta struct {
	Data
	Meta interface{} `json:"meta,omitempty"`
}

func bindUsersApi(app core.App, rg *echo.Group) {
	api := usersApi{app: app}

	subGroup := rg.Group("/users", RequireAdminAuth())
	subGroup.GET("/", api.listUsers)
	subGroup.GET("/:id", api.getUser)
	subGroup.DELETE("/:id", api.deleteUser)
	subGroup.POST("/", api.postUser)
	subGroup.PATCH("/", api.patchUser)
}

// @Summary List users
// @Tags user
// @Description Get list of the users
// @Security ApiKeyAuth
// @Router /users [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string string "search item"
// @Success 200 {object} DataMeta{data=[]UserDataID{},meta=Meta{}}
// @failure 400 {object} Error{}
// @failure 500 {object} Error{}
func (api *usersApi) listUsers(c echo.Context) error {
	users := []UserDataID{}

	meta := &UserMetaAdmin{
		Meta:  Meta{Limit: 20},
		Admin: false,
	}

	if err := c.Bind(meta); err != nil {
		return c.JSON(http.StatusBadRequest, Error{
			Error: err.Error(),
		})
	}

	reg, err := registry.Get(c.Get("registry").(string))
	if err != nil {
		return err
	}
	query := reg.DB.WithContext(c.Request().Context()).Model(&models.User{}).Limit(meta.Limit).Offset(meta.Offset)

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	result := query.Find(&users)

	// check write error
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: result.Error.Error(),
		})
	}

	// get counts
	query = reg.DB.WithContext(c.Request().Context()).Model(&models.User{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.JSON(http.StatusOK, DataMeta{
		Meta: meta.Meta,
		Data: Data{Data: users},
	})
}

// @Summary Get user
// @Tags user
// @Description Get one user with id or name
// @Security ApiKeyAuth
// @Router /user [get]
// @Param id query string false "get by id"
// @Success 200 {object} Data{data=UserDataID{}}
// @failure 400 {object} Error{}
// @failure 404 {object} Error{}
// @failure 500 {object} Error{}
func (api *usersApi) getUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: models.ErrRequiredIDName.Error(),
		})
	}

	user := new(UserDataID)

	reg, err := registry.Get(c.Get("registry").(string))
	if err != nil {
		return err
	}

	query := reg.DB.WithContext(c.Request().Context()).Model(&models.User{})
	if id != "" {
		query = query.Where("id = ?", id)
	}

	result := query.First(&user)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, Error{
			Error: result.Error.Error(),
		})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: result.Error.Error(),
		})
	}

	return c.JSON(http.StatusOK, Data{
		Data: user,
	})
}

// @Summary Delete user
// @Tags user
// @Description Delete with id or name
// @Security ApiKeyAuth
// @Router /user [delete]
// @Param id query string false "get by id"
// @Success 204 "No Content"
// @failure 400 {object} Error{}
// @failure 404 {object} Error{}
// @failure 500 {object} Error{}
func (api *usersApi) deleteUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: models.ErrRequiredIDName.Error(),
		})
	}

	reg, err := registry.Get(c.Get("registry").(string))
	if err != nil {
		return err
	}

	query := reg.DB.WithContext(c.Request().Context())
	if id != "" {
		query = query.Where("id = ?", id)
	}

	// delete directly in DB
	result := query.Unscoped().Delete(&models.User{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, Error{
			Error: "not found any related data",
		})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: result.Error.Error(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// @Summary New user
// @Tags user
// @Description Send and record new user
// @Security ApiKeyAuth
// @Router /user [post]
// @Param payload body models.UserPure{} false "send user object"
// @Success 200 {object} Data{data=ID{}}
// @failure 400 {object} Error{}
// @failure 409 {object} Error{}
// @failure 500 {object} Error{}
func (api *usersApi) postUser(c echo.Context) error {
	body := new(models.UserPure)
	if err := c.Bind(body); err != nil {
		return c.JSON(http.StatusBadRequest, Error{
			Error: err.Error(),
		})
	}

	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: "name is required",
		})
	}

	if body.Password == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: "password is required",
		})
	}

	// hash password
	if hashedPassword, err := HashPassword([]byte(body.Password)); err != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: err.Error(),
		})
	} else {
		body.Password = string(hashedPassword)
	}

	reg, err := registry.Get(c.Get("registry").(string))
	if err != nil {
		return err
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: err.Error(),
		})
	}

	result := reg.DB.WithContext(c.Request().Context()).Create(&models.User{
		UserPure: *body,
		ModelCU: models.ModelCU{
			ID: models.ID{ID: id},
		},
	})

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, Error{
			Error: result.Error.Error(),
		})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: result.Error.Error(),
		})
	}

	return c.JSON(http.StatusOK, Data{
		Data: ID{ID: id},
	})
}

func (api *usersApi) patchUser(c echo.Context) error {
	body := make(map[string]interface{})
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, Error{
			Error: err.Error(),
		})
	}

	if v, ok := body["id"].(string); !ok || v == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: "id is required and cannot be empty",
		})
	}

	// hash password
	if v, ok := body["password"].(string); ok {
		if hashedPassword, err := HashPassword([]byte(v)); err != nil {
			return c.JSON(http.StatusInternalServerError, Error{
				Error: err.Error(),
			})
		} else {
			body["password"] = hashedPassword
		}
	}

	if body["groups"] != nil {
		groupsJSON, err := json.Marshal(body["groups"])
		if err != nil {
			return c.JSON(http.StatusInternalServerError, Error{
				Error: err.Error(),
			})
		}
		body["groups"] = groupsJSON
	}

	reg, err := registry.Get(c.Get("registry").(string))
	if err != nil {
		return err
	}

	query := reg.DB.WithContext(c.Request().Context()).Model(&models.User{}).Where("id = ?", body["id"])

	result := query.Updates(body)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, Error{
			Error: result.Error.Error(),
		})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, Error{
			Error: result.Error.Error(),
		})
	}

	resultData := make(map[string]interface{})
	resultData["id"] = body["id"]

	return c.JSON(http.StatusOK, Data{
		Data: resultData,
	})
}

type usersApi struct {
	app core.App
}
