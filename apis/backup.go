package apis

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

// swagger:models BackupFileInfo
type BackupFileInfo struct {
	Key      string `json:"key"`
	Size     int64  `json:"size"`
	Modified struct {
		t time.Time
	} `json:"modified"`
}

// swagger:models BackupCreateRequest
type BackupCreateRequest struct {
	app core.App
	ctx context.Context

	Name string `form:"name" json:"name"`
}

// bindBackupApi registers the file api endpoints and the corresponding handlers.
//
//	@todo	add hooks once the app hooks api restructuring is finalized
func bindBackupApi(app core.App, rg *echo.Group) {
	api := backupApi{app: app}

	subGroup := rg.Group("/backups", ActivityLogger(app))

	subGroup.GET("", api.list, RequireAdminAuth())
	subGroup.POST("", api.create, RequireAdminAuth())
	subGroup.GET("/:key", api.download)
	subGroup.DELETE("/:key", api.delete, RequireAdminAuth())
	subGroup.POST("/:key/restore", api.restore, RequireAdminAuth())
}

type backupApi struct {
	app core.App
}

//	@Summary		Получение списка резервных копий
//	@Description	Возвращает список доступных резервных копий
//	@Tags			Backups
//	@Produce		json
//	@Security		AdminAuth
//	@Success		200	{array}		BackupFileInfo
//	@Failure		400	{string}	string	"Failed to authenticate."
//	@Router			/backups [get]
func (api *backupApi) list(c echo.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fsys, err := api.app.NewBackupsFilesystem()
	if err != nil {
		return NewBadRequestError("Failed to load backups filesystem.", err)
	}
	defer fsys.Close()

	fsys.SetContext(ctx)

	backups, err := fsys.List("")
	if err != nil {
		return NewBadRequestError("Failed to retrieve backup items. Raw error: \n"+err.Error(), nil)
	}

	result := make([]models.BackupFileInfo, len(backups))

	for i, obj := range backups {
		modified, _ := types.ParseDateTime(obj.ModTime)

		result[i] = models.BackupFileInfo{
			Key:      obj.Key,
			Size:     obj.Size,
			Modified: modified,
		}
	}

	return c.JSON(http.StatusOK, result)
}

//	@Summary		Создание резервной копии
//	@Description	Создает новую резервную копию
//	@Tags			Backups
//	@Accept			json
//	@Param			body	body	BackupCreateRequest	true	"Данные для создания резервной копии"
//	@Security		AdminAuth
//	@Success		204	"No Content"
//	@Failure		400	{string}	string	"Failed to authenticate."
//	@Router			/backups [post]
func (api *backupApi) create(c echo.Context) error {
	if api.app.Cache().Has(core.CacheKeyActiveBackup) {
		return NewBadRequestError("Try again later - another backup/restore process has already been started", nil)
	}

	form := forms.NewBackupCreate(api.app)
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	return form.Submit(func(next forms.InterceptorNextFunc[string]) forms.InterceptorNextFunc[string] {
		return func(name string) error {
			if err := next(name); err != nil {
				return NewBadRequestError("Failed to create backup.", err)
			}

			// we don't retrieve the generated backup file because it may not be
			// available yet due to the eventually consistent nature of some S3 providers
			return c.NoContent(http.StatusNoContent)
		}
	})
}

//	@Summary		Загрузка резервной копии
//	@Description	Загружает резервную копию по указанному ключу
//	@Tags			Backups
//	@Param			key		path	string	true	"Ключ резервной копии"
//	@Param			token	query	string	true	"Токен доступа"
//	@Security		AdminAuth
//	@Success		200	"OK"
//	@Failure		400	{string}	string	"Failed to authenticate."
//	@Failure		400	{string}	string	"Not exists."
//	@Router			/backups/{key} [get]
func (api *backupApi) download(c echo.Context) error {
	fileToken := c.QueryParam("token")

	_, err := api.app.Dao().FindAdminByToken(
		fileToken,
		api.app.Settings().AdminFileToken.Secret,
	)
	if err != nil {
		return NewForbiddenError("Insufficient permissions to access the resource.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fsys, err := api.app.NewBackupsFilesystem()
	if err != nil {
		return NewBadRequestError("Failed to load backups filesystem.", err)
	}
	defer fsys.Close()

	fsys.SetContext(ctx)

	key := c.PathParam("key")

	br, err := fsys.GetFile(key)
	if err != nil {
		return NewBadRequestError("Failed to retrieve backup item. Raw error: \n"+err.Error(), nil)
	}
	defer br.Close()

	return fsys.Serve(
		c.Response(),
		c.Request(),
		key,
		filepath.Base(key), // without the path prefix (if any)
	)
}

//	@Summary		Восстановление резервной копии
//	@Description	Запускает процесс восстановления резервной копии по указанному ключу
//	@Tags			Backups
//	@Param			key	path	string	true	"Ключ резервной копии"
//	@Security		AdminAuth
//	@Success		204	"No Content"
//	@Failure		400	{string}	string	"Failed to authenticate."
//	@Router			/backups/{key}/restore [post]
func (api *backupApi) restore(c echo.Context) error {
	if api.app.Cache().Has(core.CacheKeyActiveBackup) {
		return NewBadRequestError("Try again later - another backup/restore process has already been started.", nil)
	}

	//	@todo	remove the extra unescape after https://github.com/labstack/echo/issues/2447
	key, _ := url.PathUnescape(c.PathParam("key"))

	existsCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fsys, err := api.app.NewBackupsFilesystem()
	if err != nil {
		return NewBadRequestError("Failed to load backups filesystem.", err)
	}
	defer fsys.Close()

	fsys.SetContext(existsCtx)

	if exists, err := fsys.Exists(key); !exists {
		return NewBadRequestError("Missing or invalid backup file.", err)
	}

	go func() {
		// wait max 15 minutes to fetch the backup
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// give some optimistic time to write the response
		time.Sleep(1 * time.Second)

		if err := api.app.RestoreBackup(ctx, key); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}

//	@Summary		Удаление резервной копии
//	@Description	Удаляет резервную копию по указанному ключу
//	@Tags			Backups
//	@Param			key	path	string	true	"Ключ резервной копии"
//	@Security		AdminAuth
//	@Success		204	"No Content"
//	@Failure		400	{string}	string	"Failed to authenticate."
//	@Router			/backups/{key} [delete]
func (api *backupApi) delete(c echo.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fsys, err := api.app.NewBackupsFilesystem()
	if err != nil {
		return NewBadRequestError("Failed to load backups filesystem.", err)
	}
	defer fsys.Close()

	fsys.SetContext(ctx)

	key := c.PathParam("key")

	if key != "" && cast.ToString(api.app.Cache().Get(core.CacheKeyActiveBackup)) == key {
		return NewBadRequestError("The backup is currently being used and cannot be deleted.", nil)
	}

	if err := fsys.Delete(key); err != nil {
		return NewBadRequestError("Invalid or already deleted backup file. Raw error: \n"+err.Error(), nil)
	}

	return c.NoContent(http.StatusNoContent)
}
