package middleware

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Unknwon/macaron"
	"github.com/macaron-contrib/session"

	"github.com/torkelo/grafana-pro/pkg/bus"
	"github.com/torkelo/grafana-pro/pkg/log"
	m "github.com/torkelo/grafana-pro/pkg/models"
	"github.com/torkelo/grafana-pro/pkg/setting"
)

type Context struct {
	*macaron.Context
	*m.SignedInUser

	Session session.Store

	IsSignedIn bool
}

func GetContextHandler() macaron.Handler {
	return func(c *macaron.Context, sess session.Store) {
		ctx := &Context{
			Context: c,
			Session: sess,
		}

		// try get account id from request
		if userId := getRequestUserId(ctx); userId != 0 {
			query := m.GetSignedInUserQuery{UserId: userId}
			if err := bus.Dispatch(&query); err != nil {
				log.Error(3, "Failed to get user by id, %v, %v", userId, err)
			} else {
				ctx.IsSignedIn = true
				ctx.SignedInUser = query.Result
			}
		} else if key := getApiKey(ctx); key != "" {
			// Try API Key auth
			keyQuery := m.GetApiKeyByKeyQuery{Key: key}
			if err := bus.Dispatch(&keyQuery); err != nil {
				ctx.JsonApiErr(401, "Invalid API key", err)
				return
			} else {
				keyInfo := keyQuery.Result

				ctx.IsSignedIn = true
				ctx.SignedInUser = &m.SignedInUser{}

				// TODO: fix this
				ctx.AccountRole = keyInfo.Role
				ctx.ApiKeyId = keyInfo.Id
				ctx.AccountId = keyInfo.AccountId
			}
		}

		c.Map(ctx)
	}
}

// Handle handles and logs error by given status.
func (ctx *Context) Handle(status int, title string, err error) {
	if err != nil {
		log.Error(4, "%s: %v", title, err)
		if macaron.Env != macaron.PROD {
			ctx.Data["ErrorMsg"] = err
		}
	}

	switch status {
	case 404:
		ctx.Data["Title"] = "Page Not Found"
	case 500:
		ctx.Data["Title"] = "Internal Server Error"
	}

	ctx.HTML(status, strconv.Itoa(status))
}

func (ctx *Context) JsonOK(message string) {
	resp := make(map[string]interface{})

	resp["message"] = message

	ctx.JSON(200, resp)
}

func (ctx *Context) IsApiRequest() bool {
	return strings.HasPrefix(ctx.Req.URL.Path, "/api")
}

func (ctx *Context) JsonApiErr(status int, message string, err error) {
	resp := make(map[string]interface{})

	if err != nil {
		log.Error(4, "%s: %v", message, err)
		if setting.Env != setting.PROD {
			resp["error"] = err.Error()
		}
	}

	switch status {
	case 404:
		resp["message"] = "Not Found"
	case 500:
		resp["message"] = "Internal Server Error"
	}

	if message != "" {
		resp["message"] = message
	}

	ctx.JSON(status, resp)
}

func (ctx *Context) JsonBody(model interface{}) bool {
	b, _ := ctx.Req.Body().Bytes()
	err := json.Unmarshal(b, &model)
	return err == nil
}
