package apis

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/search"
)

// bindCollectionApi registers the collection api endpoints and the corresponding handlers.
func bindCollectionApi(app core.App, rg *echo.Group) {
	api := collectionApi{app: app}

	subGroup := rg.Group("/collections", ActivityLogger(app), RequireAdminAuth())
	// @Summary Получить список коллекций
	// @Description Возвращает список коллекций с возможностью фильтрации и сортировки
	// @Tags Collections
	// @Param id query string false "ID коллекции"
	// @Param created query string false "Дата создания коллекции в формате ISO8601"
	// @Param updated query string false "Дата обновления коллекции в формате ISO8601"
	// @Param name query string false "Название коллекции"
	// @Param system query boolean false "Системная коллекция"
	// @Param type query string false "Тип коллекции"
	// @Security AdminAuth
	// @Success 200 {object} SearchResult{Collections} "OK"
	// @Failure 400 {object} ErrorResponse
	// @Router /collections [get]
	subGroup.GET("", api.list)

	// @Summary Создать коллекцию
	// @Description Создает новую коллекцию
	// @Tags Collections
	// @Accept json
	// @Produce json
	// @Param collection body CollectionCreateRequest true "Данные для создания коллекции"
	// @Security AdminAuth
	// @Success 200 {object} Collection "OK"
	// @Failure 400 {object} ErrorResponse
	// @Router /collections [post]
	subGroup.POST("", api.create)

	// @Summary Просмотреть коллекцию
	// @Description Возвращает информацию о коллекции по ее имени или ID
	// @Tags Collections
	// @Accept json
	// @Produce json
	// @Param collection path string true "Имя или ID коллекции"
	// @Security AdminAuth
	// @Success 200 {object} Collection "OK"
	// @Failure 400 {object} ErrorResponse
	// @Failure 404 {object} ErrorResponse
	// @Router /collections/{collection} [get]
	subGroup.GET("/:collection", api.view)

	// @Summary Обновить коллекцию
	// @Description Обновляет информацию о коллекции по ее имени или ID
	// @Tags Collections
	// @Accept json
	// @Produce json
	// @Param collection path string true "Имя или ID коллекции"
	// @Param body body CollectionUpsertRequest true "Данные для обновления коллекции"
	// @Security AdminAuth
	// @Success 200 {object} Collection "OK"
	// @Failure 400 {object} ErrorResponse
	// @Failure 404 {object} ErrorResponse
	// @Router /collections/{collection} [patch]
	subGroup.PATCH("/:collection", api.update)

	// @Summary Удалить коллекцию
	// @Description Удаляет коллекцию по ее имени или ID
	// @Tags Collections
	// @Param collection path string true "Имя или ID коллекции"
	// @Security AdminAuth
	// @Success 204 "No Content"
	// @Failure 404 {object} ErrorResponse
	// @Router /collections/{collection} [delete]
	subGroup.DELETE("/:collection", api.delete)

	// @Summary Импортировать коллекции
	// @Description Импортирует коллекции из переданных данных
	// @Tags Collections
	// @Security AdminAuth
	// @Accept json
	// @Produce json
	// @Param body body CollectionsImportRequest true "Данные для импорта коллекций"
	// @Success 204 "No Content"
	// @Failure 400 {object} ErrorResponse
	// @Router /collections/import [post]
	subGroup.PUT("/import", api.bulkImport)
}

type collectionApi struct {
	app core.App
}

func (api *collectionApi) list(c echo.Context) error {
	fieldResolver := search.NewSimpleFieldResolver(
		"id", "created", "updated", "name", "system", "type",
	)

	collections := []*models.Collection{}

	result, err := search.NewProvider(fieldResolver).
		Query(api.app.Dao().CollectionQuery()).
		ParseAndExec(c.QueryParams().Encode(), &collections)

	if err != nil {
		return NewBadRequestError("", err)
	}

	event := new(core.CollectionsListEvent)
	event.HttpContext = c
	event.Collections = collections
	event.Result = result

	return api.app.OnCollectionsListRequest().Trigger(event, func(e *core.CollectionsListEvent) error {
		return e.HttpContext.JSON(http.StatusOK, e.Result)
	})
}

func (api *collectionApi) view(c echo.Context) error {
	collection, err := api.app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil || collection == nil {
		return NewNotFoundError("", err)
	}

	event := new(core.CollectionViewEvent)
	event.HttpContext = c
	event.Collection = collection

	return api.app.OnCollectionViewRequest().Trigger(event, func(e *core.CollectionViewEvent) error {
		return e.HttpContext.JSON(http.StatusOK, e.Collection)
	})
}

func (api *collectionApi) create(c echo.Context) error {
	collection := &models.Collection{}

	form := forms.NewCollectionUpsert(api.app, collection)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	event := new(core.CollectionCreateEvent)
	event.HttpContext = c
	event.Collection = collection

	// create the collection
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Collection]) forms.InterceptorNextFunc[*models.Collection] {
		return func(m *models.Collection) error {
			event.Collection = m

			return api.app.OnCollectionBeforeCreateRequest().Trigger(event, func(e *core.CollectionCreateEvent) error {
				if err := next(e.Collection); err != nil {
					return NewBadRequestError("Failed to create the collection.", err)
				}

				return e.HttpContext.JSON(http.StatusOK, e.Collection)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnCollectionAfterCreateRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *collectionApi) update(c echo.Context) error {
	collection, err := api.app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil || collection == nil {
		return NewNotFoundError("", err)
	}

	form := forms.NewCollectionUpsert(api.app, collection)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	event := new(core.CollectionUpdateEvent)
	event.HttpContext = c
	event.Collection = collection

	// update the collection
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*models.Collection]) forms.InterceptorNextFunc[*models.Collection] {
		return func(m *models.Collection) error {
			event.Collection = m

			return api.app.OnCollectionBeforeUpdateRequest().Trigger(event, func(e *core.CollectionUpdateEvent) error {
				if err := next(e.Collection); err != nil {
					return NewBadRequestError("Failed to update the collection.", err)
				}

				return e.HttpContext.JSON(http.StatusOK, e.Collection)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnCollectionAfterUpdateRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}

func (api *collectionApi) delete(c echo.Context) error {
	collection, err := api.app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil || collection == nil {
		return NewNotFoundError("", err)
	}

	event := new(core.CollectionDeleteEvent)
	event.HttpContext = c
	event.Collection = collection

	handlerErr := api.app.OnCollectionBeforeDeleteRequest().Trigger(event, func(e *core.CollectionDeleteEvent) error {
		if err := api.app.Dao().DeleteCollection(e.Collection); err != nil {
			return NewBadRequestError("Failed to delete collection due to existing dependency.", err)
		}

		return e.HttpContext.NoContent(http.StatusNoContent)
	})

	if handlerErr == nil {
		if err := api.app.OnCollectionAfterDeleteRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return handlerErr
}

func (api *collectionApi) bulkImport(c echo.Context) error {
	form := forms.NewCollectionsImport(api.app)

	// load request data
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	event := new(core.CollectionsImportEvent)
	event.HttpContext = c
	event.Collections = form.Collections

	// import collections
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[[]*models.Collection]) forms.InterceptorNextFunc[[]*models.Collection] {
		return func(imports []*models.Collection) error {
			event.Collections = imports

			return api.app.OnCollectionsBeforeImportRequest().Trigger(event, func(e *core.CollectionsImportEvent) error {
				if err := next(e.Collections); err != nil {
					return NewBadRequestError("Failed to import the submitted collections.", err)
				}

				return e.HttpContext.NoContent(http.StatusNoContent)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnCollectionsAfterImportRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}
