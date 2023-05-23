package apis

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tokens"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/search"

	// "github.com/swaggo/echo-swagger"
	_ "github.com/swaggo/echo-swagger/example/docs"
)

// bindAdminApi registers the admin api endpoints and the corresponding handlers.
func bindAdminApi(app core.App, rg *echo.Group) {
	api := adminApi{app: app}

	subGroup := rg.Group("/admins", ActivityLogger(app))
	// ShowAccount godoc
	//	@Summary		Аутентификация администратора с использованием пароля
	//	@Description	Выполняет аутентификацию администратора с использованием пароля
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			adminLogin	body			forms.AdminLogin	true	"Данные аутентификации администратора"
	//	@Success		200			"No Content"	AuthResponse
	//	@Failure		400			{object}		error
	//	@Router			/admins/auth-with-password [post]
	subGroup.POST("/auth-with-password", api.authWithPassword)

	//	@Summary		Запрос на сброс пароля администратора
	//	@Description	Отправляет запрос на сброс пароля администратора
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			passwordResetRequest	body	AdminPasswordResetRequestForm	true	"Данные запроса на сброс пароля администратора"
	//	@Success		204						"No Content"
	//	@Failure		400						{object}	ErrorResponse
	//	@Router			/admins/request-password-reset [post]
	subGroup.POST("/request-password-reset", api.requestPasswordReset)

	//	@Summary		Подтверждение сброса пароля администратора
	//	@Description	Подтверждает сброс пароля администратора
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			passwordResetConfirm	body	AdminPasswordResetConfirmForm	true	"Данные подтверждения сброса пароля администратора"
	//	@Success		204						"No Content"
	//	@Failure		400						{object}	ErrorResponse
	//	@Router			/admins/confirm-password-reset [post]
	subGroup.POST("/confirm-password-reset", api.confirmPasswordReset)

	//	@Summary		Обновление авторизации администратора
	//	@Description	Обновляет токен авторизации администратора
	//	@Tags			Admin
	//	@Security		AdminAuth
	//	@Accept			json
	//	@Produce		json
	//	@Success		200	{object}	AuthResponse
	//	@Failure		401	{object}	ErrorResponse
	//	@Router			/admins/auth-refresh [post]
	subGroup.POST("/auth-refresh", api.authRefresh, RequireAdminAuth())

	//	@Summary		Получение списка администраторов
	//	@Description	Возвращает список администраторов с возможностью поиска и сортировки
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			id		query		string	false	"Идентификатор администратора"
	//	@Param			created	query		string	false	"Дата создания администратора"
	//	@Param			updated	query		string	false	"Дата обновления администратора"
	//	@Param			name	query		string	false	"Имя администратора"
	//	@Param			email	query		string	false	"Email администратора"
	//	@Success		200		{array}		Admin
	//	@Failure		400		{object}	ErrorResponse
	//	@Router			/admins [get]
	subGroup.GET("", api.list, RequireAdminAuth())

	//	@Summary		Создание администратора
	//	@Description	Создает нового администратора
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			admin	body		AdminCreateForm	true	"Данные для создания администратора"
	//	@Success		200		{object}	Admin
	//	@Failure		400		{object}	ErrorResponse
	//	@Router			/admins [post]
	subGroup.POST("", api.create, RequireAdminAuthOnlyIfAny(app))

	//	@Summary		Просмотр администратора
	//	@Description	Возвращает информацию об указанном администраторе по его идентификатору
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			id	path		string	true	"Идентификатор администратора"
	//	@Success		200	{object}	Admin
	//	@Failure		404	{object}	ErrorResponse
	//	@Router			/admins/{id} [get]
	subGroup.GET("/:id", api.view, RequireAdminAuth())

	//	@Summary		Обновление администратора
	//	@Description	Обновляет информацию об указанном администраторе по его идентификатору
	//	@Tags			Admin
	//	@Accept			json
	//	@Produce		json
	//	@Param			id		path		string			true	"Идентификатор администратора"
	//	@Param			admin	body		AdminUpdateForm	true	"Данные для обновления администратора"
	//	@Success		200		{object}	Admin
	//	@Failure		400		{object}	ErrorResponse
	//	@Failure		404		{object}	ErrorResponse
	//	@Router			/admins/{id} [patch]
	subGroup.PATCH("/:id", api.update, RequireAdminAuth())

	//	@Summary		Удаление администратора
	//	@Description	Удаляет указанного администратора по его идентификатору
	//	@Tags			Admin
	//	@Produce		plain
	//	@Param			id	path	string	true	"Идентификатор администратора"
	//	@Success		204	"No Content"
	//	@Failure		404	{object}	ErrorResponse
	//	@Router			/admins/{id} [delete]
	subGroup.DELETE("/:id", api.delete, RequireAdminAuth())
}

type adminApi struct {
	app core.App
}

func (api *adminApi) authResponse(c echo.Context, admin *models.Admin) error {
	token, tokenErr := tokens.NewAdminAuthToken(api.app, admin)
	if tokenErr != nil {
		return NewBadRequestError("Failed to create auth token.", tokenErr)
	}

	event := new(core.AdminAuthEvent)
	event.HttpContext = c
	event.Admin = admin
	event.Token = token

	return api.app.OnAdminAuthRequest().Trigger(event, func(e *core.AdminAuthEvent) error {
		return e.HttpContext.JSON(200, map[string]any{
			"token": e.Token,
			"admin": e.Admin,
		})
	})
}

func (api *adminApi) authRefresh(c echo.Context) error {
	admin, _ := c.Get(ContextAdminKey).(*models.Admin)
	if admin == nil {
		return NewNotFoundError("Missing auth admin context.", nil)
	}

	event := new(core.AdminAuthRefreshEvent)
	event.HttpContext = c
	event.Admin = admin

	handlerErr := api.app.OnAdminBeforeAuthRefreshRequest().Trigger(event, func(e *core.AdminAuthRefreshEvent) error {
		return api.authResponse(e.HttpContext, e.Admin)
	})

	if handlerErr == nil {
		if err := api.app.OnAdminAfterAuthRefreshRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return handlerErr
}

func (api *adminApi) authWithPassword(c echo.Context) error {
	form := forms.NewAdminLogin(api.app)
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	event := new(core.AdminAuthWithPasswordEvent)
	event.HttpContext = c
	event.Password = form.Password
	event.Identity = form.Identity

	_, submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Admin]) forms.InterceptorNextFunc[*models.Admin] {
		return func(admin *models.Admin) error {
			event.Admin = admin

			return api.app.OnAdminBeforeAuthWithPasswordRequest().Trigger(event, func(e *core.AdminAuthWithPasswordEvent) error {
				if err := next(e.Admin); err != nil {
					return NewBadRequestError("Failed to authenticate.", err)
				}

				return api.authResponse(e.HttpContext, e.Admin)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnAdminAfterAuthWithPasswordRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *adminApi) requestPasswordReset(c echo.Context) error {
	form := forms.NewAdminPasswordResetRequest(api.app)
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	if err := form.Validate(); err != nil {
		return NewBadRequestError("An error occurred while validating the form.", err)
	}

	event := new(core.AdminRequestPasswordResetEvent)
	event.HttpContext = c

	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Admin]) forms.InterceptorNextFunc[*models.Admin] {
		return func(Admin *models.Admin) error {
			event.Admin = Admin

			return api.app.OnAdminBeforeRequestPasswordResetRequest().Trigger(event, func(e *core.AdminRequestPasswordResetEvent) error {
				// run in background because we don't need to show the result to the client
				routine.FireAndForget(func() {
					if err := next(e.Admin); err != nil && api.app.IsDebug() {
						log.Println(err)
					}
				})

				return e.HttpContext.NoContent(http.StatusNoContent)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnAdminAfterRequestPasswordResetRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	} else if api.app.IsDebug() {
		log.Println(submitErr)
	}

	// don't return the response error to prevent emails enumeration
	if !c.Response().Committed {
		c.NoContent(http.StatusNoContent)
	}

	return nil
}

func (api *adminApi) confirmPasswordReset(c echo.Context) error {
	form := forms.NewAdminPasswordResetConfirm(api.app)
	if readErr := c.Bind(form); readErr != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", readErr)
	}

	event := new(core.AdminConfirmPasswordResetEvent)
	event.HttpContext = c

	_, submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Admin]) forms.InterceptorNextFunc[*models.Admin] {
		return func(admin *models.Admin) error {
			event.Admin = admin

			return api.app.OnAdminBeforeConfirmPasswordResetRequest().Trigger(event, func(e *core.AdminConfirmPasswordResetEvent) error {
				if err := next(e.Admin); err != nil {
					return NewBadRequestError("Failed to set new password.", err)
				}

				return e.HttpContext.NoContent(http.StatusNoContent)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnAdminAfterConfirmPasswordResetRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *adminApi) list(c echo.Context) error {
	fieldResolver := search.NewSimpleFieldResolver(
		"id", "created", "updated", "name", "email",
	)

	admins := []*models.Admin{}

	result, err := search.NewProvider(fieldResolver).
		Query(api.app.Dao().AdminQuery()).
		ParseAndExec(c.QueryParams().Encode(), &admins)

	if err != nil {
		return NewBadRequestError("", err)
	}

	event := new(core.AdminsListEvent)
	event.HttpContext = c
	event.Admins = admins
	event.Result = result

	return api.app.OnAdminsListRequest().Trigger(event, func(e *core.AdminsListEvent) error {
		return e.HttpContext.JSON(http.StatusOK, e.Result)
	})
}

func (api *adminApi) view(c echo.Context) error {
	id := c.PathParam("id")
	if id == "" {
		return NewNotFoundError("", nil)
	}

	admin, err := api.app.Dao().FindAdminById(id)
	if err != nil || admin == nil {
		return NewNotFoundError("", err)
	}

	event := new(core.AdminViewEvent)
	event.HttpContext = c
	event.Admin = admin

	return api.app.OnAdminViewRequest().Trigger(event, func(e *core.AdminViewEvent) error {
		return e.HttpContext.JSON(http.StatusOK, e.Admin)
	})
}

func (api *adminApi) create(c echo.Context) error {
	admin := &models.Admin{}

	form := forms.NewAdminUpsert(api.app, admin)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	event := new(core.AdminCreateEvent)
	event.HttpContext = c
	event.Admin = admin

	// create the admin
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Admin]) forms.InterceptorNextFunc[*models.Admin] {
		return func(m *models.Admin) error {
			event.Admin = m

			return api.app.OnAdminBeforeCreateRequest().Trigger(event, func(e *core.AdminCreateEvent) error {
				if err := next(e.Admin); err != nil {
					return NewBadRequestError("Failed to create admin.", err)
				}

				return e.HttpContext.JSON(http.StatusOK, e.Admin)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnAdminAfterCreateRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *adminApi) update(c echo.Context) error {
	id := c.PathParam("id")
	if id == "" {
		return NewNotFoundError("", nil)
	}

	admin, err := api.app.Dao().FindAdminById(id)
	if err != nil || admin == nil {
		return NewNotFoundError("", err)
	}

	form := forms.NewAdminUpsert(api.app, admin)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	event := new(core.AdminUpdateEvent)
	event.HttpContext = c
	event.Admin = admin

	// update the admin
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Admin]) forms.InterceptorNextFunc[*models.Admin] {
		return func(m *models.Admin) error {
			event.Admin = m

			return api.app.OnAdminBeforeUpdateRequest().Trigger(event, func(e *core.AdminUpdateEvent) error {
				if err := next(e.Admin); err != nil {
					return NewBadRequestError("Failed to update admin.", err)
				}

				return e.HttpContext.JSON(http.StatusOK, e.Admin)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnAdminAfterUpdateRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *adminApi) delete(c echo.Context) error {
	id := c.PathParam("id")
	if id == "" {
		return NewNotFoundError("", nil)
	}

	admin, err := api.app.Dao().FindAdminById(id)
	if err != nil || admin == nil {
		return NewNotFoundError("", err)
	}

	event := new(core.AdminDeleteEvent)
	event.HttpContext = c
	event.Admin = admin

	handlerErr := api.app.OnAdminBeforeDeleteRequest().Trigger(event, func(e *core.AdminDeleteEvent) error {
		if err := api.app.Dao().DeleteAdmin(e.Admin); err != nil {
			return NewBadRequestError("Failed to delete admin.", err)
		}

		return e.HttpContext.NoContent(http.StatusNoContent)
	})

	if handlerErr == nil {
		if err := api.app.OnAdminAfterDeleteRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return handlerErr
}
