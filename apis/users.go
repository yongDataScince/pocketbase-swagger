package apis

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"gorm.io/gorm"
)

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
	/*
		router.Get("/user", middleware.JWTCheck([]string{"admin"}, middleware.IDFromQuery), getUser)
		router.Post("/user", middleware.JWTCheck([]string{"admin"}, nil), postUser)
		router.Patch("/user", middleware.JWTCheck([]string{"admin"}, middleware.IDFromBody), patchUser)
		router.Delete("/user", middleware.JWTCheck([]string{"admin"}, middleware.IDFromQuery), deleteUser)
	*/
}

// @Summary List users
// @Tags user
// @Description Get list of the users
// @Security ApiKeyAuth
// @Router /users [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string string "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]UserDataID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
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

	reg := registry.Reg().Get(c.Get("registry").(string))

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
// @Success 200 {object} apimodels.Data{data=UserDataID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func (api *usersApi) getUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: models.ErrRequiredIDName.Error(),
		})
	}

	user := new(UserDataID)

	reg := registry.Reg().Get(c.Get("registry").(string))

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
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func (api *usersApi) deleteUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, Error{
			Error: models.ErrRequiredIDName.Error(),
		})
	}

	reg := registry.Reg().Get(c.Get("registry").(string))

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

type usersApi struct {
	app core.App
}
