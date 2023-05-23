package apis

import (
	"log"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models/settings"
)

type EmailTemplate struct {
	Body      string `form:"body" json:"body"`
	Subject   string `form:"subject" json:"subject"`
	ActionUrl string `form:"actionUrl" json:"actionUrl"`
}

type LogsConfig struct {
	MaxDays int `form:"maxDays" json:"maxDays"`
}

type TokenConfig struct {
	Secret   string `form:"secret" json:"secret"`
	Duration int64  `form:"duration" json:"duration"`
}

type SmtpConfig struct {
	Enabled  bool   `form:"enabled" json:"enabled"`
	Host     string `form:"host" json:"host"`
	Port     int    `form:"port" json:"port"`
	Username string `form:"username" json:"username"`
	Password string `form:"password" json:"password"`

	// SMTP AUTH - PLAIN (default) or LOGIN
	AuthMethod string `form:"authMethod" json:"authMethod"`

	// Whether to enforce TLS encryption for the mail server connection.
	//
	// When set to false StartTLS command is send, leaving the server
	// to decide whether to upgrade the connection or not.
	Tls bool `form:"tls" json:"tls"`
}

type S3Config struct {
	Enabled        bool   `form:"enabled" json:"enabled"`
	Bucket         string `form:"bucket" json:"bucket"`
	Region         string `form:"region" json:"region"`
	Endpoint       string `form:"endpoint" json:"endpoint"`
	AccessKey      string `form:"accessKey" json:"accessKey"`
	Secret         string `form:"secret" json:"secret"`
	ForcePathStyle bool   `form:"forcePathStyle" json:"forcePathStyle"`
}

type BackupsConfig struct {
	// Cron is a cron expression to schedule auto backups, eg. "* * * * *".
	//
	// Leave it empty to disable the auto backups functionality.
	Cron string `form:"cron" json:"cron"`

	// CronMaxKeep is the the max number of cron generated backups to
	// keep before removing older entries.
	//
	// This field works only when the cron config has valid cron expression.
	CronMaxKeep int `form:"cronMaxKeep" json:"cronMaxKeep"`

	// S3 is an optional S3 storage config specifying where to store the app backups.
	S3 S3Config `form:"s3" json:"s3"`
}

type AuthProviderConfig struct {
	Enabled      bool   `form:"enabled" json:"enabled"`
	ClientId     string `form:"clientId" json:"clientId"`
	ClientSecret string `form:"clientSecret" json:"clientSecret"`
	AuthUrl      string `form:"authUrl" json:"authUrl"`
	TokenUrl     string `form:"tokenUrl" json:"tokenUrl"`
	UserApiUrl   string `form:"userApiUrl" json:"userApiUrl"`
}

type EmailAuthConfig struct {
	Enabled           bool     `form:"enabled" json:"enabled"`
	ExceptDomains     []string `form:"exceptDomains" json:"exceptDomains"`
	OnlyDomains       []string `form:"onlyDomains" json:"onlyDomains"`
	MinPasswordLength int      `form:"minPasswordLength" json:"minPasswordLength"`
}

// swagger:models Settings
type Settings struct {
	Meta struct {
		AppName                    string        `form:"appName" json:"appName"`
		AppUrl                     string        `form:"appUrl" json:"appUrl"`
		HideControls               bool          `form:"hideControls" json:"hideControls"`
		SenderName                 string        `form:"senderName" json:"senderName"`
		SenderAddress              string        `form:"senderAddress" json:"senderAddress"`
		VerificationTemplate       EmailTemplate `form:"verificationTemplate" json:"verificationTemplate"`
		ResetPasswordTemplate      EmailTemplate `form:"resetPasswordTemplate" json:"resetPasswordTemplate"`
		ConfirmEmailChangeTemplate EmailTemplate `form:"confirmEmailChangeTemplate" json:"confirmEmailChangeTemplate"`
	} `form:"meta" json:"meta"`
	Logs    LogsConfig    `form:"logs" json:"logs"`
	Smtp    SmtpConfig    `form:"smtp" json:"smtp"`
	S3      S3Config      `form:"s3" json:"s3"`
	Backups BackupsConfig `form:"backups" json:"backups"`

	AdminAuthToken           TokenConfig `form:"adminAuthToken" json:"adminAuthToken"`
	AdminPasswordResetToken  TokenConfig `form:"adminPasswordResetToken" json:"adminPasswordResetToken"`
	AdminFileToken           TokenConfig `form:"adminFileToken" json:"adminFileToken"`
	RecordAuthToken          TokenConfig `form:"recordAuthToken" json:"recordAuthToken"`
	RecordPasswordResetToken TokenConfig `form:"recordPasswordResetToken" json:"recordPasswordResetToken"`
	RecordEmailChangeToken   TokenConfig `form:"recordEmailChangeToken" json:"recordEmailChangeToken"`
	RecordVerificationToken  TokenConfig `form:"recordVerificationToken" json:"recordVerificationToken"`
	RecordFileToken          TokenConfig `form:"recordFileToken" json:"recordFileToken"`

	// Deprecated: Will be removed in v0.9+
	EmailAuth EmailAuthConfig `form:"emailAuth" json:"emailAuth"`

	GoogleAuth    AuthProviderConfig `form:"googleAuth" json:"googleAuth"`
	FacebookAuth  AuthProviderConfig `form:"facebookAuth" json:"facebookAuth"`
	GithubAuth    AuthProviderConfig `form:"githubAuth" json:"githubAuth"`
	GitlabAuth    AuthProviderConfig `form:"gitlabAuth" json:"gitlabAuth"`
	DiscordAuth   AuthProviderConfig `form:"discordAuth" json:"discordAuth"`
	TwitterAuth   AuthProviderConfig `form:"twitterAuth" json:"twitterAuth"`
	MicrosoftAuth AuthProviderConfig `form:"microsoftAuth" json:"microsoftAuth"`
	SpotifyAuth   AuthProviderConfig `form:"spotifyAuth" json:"spotifyAuth"`
	KakaoAuth     AuthProviderConfig `form:"kakaoAuth" json:"kakaoAuth"`
	TwitchAuth    AuthProviderConfig `form:"twitchAuth" json:"twitchAuth"`
	StravaAuth    AuthProviderConfig `form:"stravaAuth" json:"stravaAuth"`
	GiteeAuth     AuthProviderConfig `form:"giteeAuth" json:"giteeAuth"`
	LivechatAuth  AuthProviderConfig `form:"livechatAuth" json:"livechatAuth"`
	GiteaAuth     AuthProviderConfig `form:"giteaAuth" json:"giteaAuth"`
	OIDCAuth      AuthProviderConfig `form:"oidcAuth" json:"oidcAuth"`
	OIDC2Auth     AuthProviderConfig `form:"oidc2Auth" json:"oidc2Auth"`
	OIDC3Auth     AuthProviderConfig `form:"oidc3Auth" json:"oidc3Auth"`
	AppleAuth     AuthProviderConfig `form:"appleAuth" json:"appleAuth"`
}

// bindSettingsApi registers the settings api endpoints.
func bindSettingsApi(app core.App, rg *echo.Group) {
	api := settingsApi{app: app}

	subGroup := rg.Group("/settings", ActivityLogger(app), RequireAdminAuth())

	subGroup.GET("", api.list)
	subGroup.PATCH("", api.set)
	subGroup.POST("/test/s3", api.testS3)
	subGroup.POST("/test/email", api.testEmail)
	subGroup.POST("/apple/generate-client-secret", api.generateAppleClientSecret)
}

type settingsApi struct {
	app core.App
}

// @Summary		Получение списка настроек
// @Description	Возвращает список всех настроек
// @Tags			Settings
// @Security		AdminAuth
// @Accept			json
// @Produce		json
// @Success		200	{object}	Settings
// @Failure		400	{string}	string	"Failed to authenticate."
// @Router			/settings [get]
func (api *settingsApi) list(c echo.Context) error {
	settings, err := api.app.Settings().RedactClone()
	if err != nil {
		return NewBadRequestError("", err)
	}

	event := new(core.SettingsListEvent)
	event.HttpContext = c
	event.RedactedSettings = settings

	return api.app.OnSettingsListRequest().Trigger(event, func(e *core.SettingsListEvent) error {
		return e.HttpContext.JSON(http.StatusOK, e.RedactedSettings)
	})
}

// swagger:models UpdateSettingsRequest
type UpdateSettingsRequest struct {
	*Settings

	app core.App
	dao *daos.Dao
}

// @Summary		Обновление настроек
// @Description	Обновляет указанные настройки
// @Tags			Settings
// @Security		AdminAuth
// @Accept			json
// @Produce		json
// @Param			body	body		UpdateSettingsRequest	true	"Данные для обновления настроек"
// @Success		200		"Обновление настроек успешно"
// @Failure		400		{string}	string	"Failed to authenticate."
// @Router			/settings [patch]
func (api *settingsApi) set(c echo.Context) error {
	form := forms.NewSettingsUpsert(api.app)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	event := new(core.SettingsUpdateEvent)
	event.HttpContext = c
	event.OldSettings = api.app.Settings()

	// update the settings
	submitErr := form.Submit(func(next forms.InterceptorNextFunc[*settings.Settings]) forms.InterceptorNextFunc[*settings.Settings] {
		return func(s *settings.Settings) error {
			event.NewSettings = s

			return api.app.OnSettingsBeforeUpdateRequest().Trigger(event, func(e *core.SettingsUpdateEvent) error {
				if err := next(e.NewSettings); err != nil {
					return NewBadRequestError("An error occurred while submitting the form.", err)
				}

				redactedSettings, err := api.app.Settings().RedactClone()
				if err != nil {
					return NewBadRequestError("", err)
				}

				return e.HttpContext.JSON(http.StatusOK, redactedSettings)
			})
		}
	})

	if submitErr == nil {
		if err := api.app.OnSettingsAfterUpdateRequest().Trigger(event); err != nil && api.app.IsDebug() {
			log.Println(err)
		}
	}

	return submitErr
}


// swagger:models TestS3SettingsRequest
type TestS3SettingsRequest struct {
	app core.App

	// The name of the filesystem - storage or backups
	Filesystem string `form:"filesystem" json:"filesystem"`
}

// @Summary		Тестирование настроек для хранилища S3
// @Description	Проверяет настройки для хранилища S3
// @Tags			Settings
// @Security		AdminAuth
// @Accept			json
// @Produce		json
// @Param			body	body		TestS3SettingsRequest	true	"Данные для тестирования настроек S3"
// @Success		200		"Тестирование настроек для хранилища S3 успешно"
// @Failure		400		{string}	string	"Failed to authenticate."
// @Router			/settings/test/s3 [post]
func (api *settingsApi) testS3(c echo.Context) error {
	form := forms.NewTestS3Filesystem(api.app)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	// send
	if err := form.Submit(); err != nil {
		// form error
		if fErr, ok := err.(validation.Errors); ok {
			return NewBadRequestError("Failed to test the S3 filesystem.", fErr)
		}

		// mailer error
		return NewBadRequestError("Failed to test the S3 filesystem. Raw error: \n"+err.Error(), nil)
	}

	return c.NoContent(http.StatusNoContent)
}

// swagger:models TestEmailSettingsRequest
type TestEmailSettingsRequest struct {
	app core.App

	Template string `form:"template" json:"template"`
	Email    string `form:"email" json:"email"`
}

// @Summary		Тестирование настроек для электронной почты
// @Description	Проверяет настройки для отправки электронной почты
// @Tags			Settings
// @Security		AdminAuth
// @Accept			json
// @Produce		json
// @Param			body	body		TestEmailSettingsRequest	true	"Данные для тестирования настроек электронной почты"
// @Success		200		"Тестирование настроек для электронной почты успешно"
// @Failure		400		{string}	string	"Failed to authenticate."
// @Router			/settings/test/email [post]
func (api *settingsApi) testEmail(c echo.Context) error {
	form := forms.NewTestEmailSend(api.app)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	// send
	if err := form.Submit(); err != nil {
		// form error
		if fErr, ok := err.(validation.Errors); ok {
			return NewBadRequestError("Failed to send the test email.", fErr)
		}

		// mailer error
		return NewBadRequestError("Failed to send the test email. Raw error: \n"+err.Error(), nil)
	}

	return c.NoContent(http.StatusNoContent)
}

// @Summary		Генерация секретного ключа для авторизации Apple
// @Description	Генерирует секретный ключ для использования при авторизации Apple
// @Tags			Settings
// @Security		AdminAuth
// @Accept			json
// @Produce		json
// @Success		200	"Генерация секретного ключа для авторизации Apple успешно"
// @Failure		400	{string}	string	"Failed to authenticate."
// @Router			/settings/apple/generate-client-secret [post]
func (api *settingsApi) generateAppleClientSecret(c echo.Context) error {
	form := forms.NewAppleClientSecretCreate(api.app)

	// load request
	if err := c.Bind(form); err != nil {
		return NewBadRequestError("An error occurred while loading the submitted data.", err)
	}

	// generate
	secret, err := form.Submit()
	if err != nil {
		// form error
		if fErr, ok := err.(validation.Errors); ok {
			return NewBadRequestError("Invalid client secret data.", fErr)
		}

		// secret generation error
		return NewBadRequestError("Failed to generate client secret. Raw error: \n"+err.Error(), nil)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"secret": secret,
	})
}
