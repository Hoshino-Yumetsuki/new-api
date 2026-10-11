package router

import (
	"bytes"
	"embed"
	"html"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets, pluginDispatcher gin.HandlerFunc) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")
	serveStatic := middleware.ServeFrontendFiles(frontendFS)
	compress := gzip.Gzip(gzip.DefaultCompression)
	type cachedPage struct {
		title, description string
		page               []byte
	}
	var pageMutex sync.Mutex
	var cached cachedPage

	router.NoRoute(
		pluginDispatcher,
		middleware.RouteTag("web"),
		func(c *gin.Context) {
			// ServeFrontendFiles sends build files already compressed.
			if c.Request.URL.Path == "/" || c.Request.URL.Path == "/index.html" || !frontendFS.Exists("/", c.Request.URL.Path) {
				compress(c)
			}
		},
		middleware.AccessTokenAudit(),
		middleware.GlobalWebRateLimit(frontendFS),
		middleware.Cache(),
		func(c *gin.Context) {
			// The embedded filesystem serves index.html directly, bypassing the
			// live metadata; other assets still use ServeFrontendFiles.
			if c.Request.URL.Path != "/" && c.Request.URL.Path != "/index.html" {
				serveStatic(c)
			}
		},
		func(c *gin.Context) {
			if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
				controller.RelayNotFound(c)
				return
			}
			isHome := c.Request.URL.Path == "/" || c.Request.URL.Path == "/index.html"
			c.Header("Cache-Control", "no-cache")
			if !isHome {
				c.Data(http.StatusOK, "text/html; charset=utf-8", assets.IndexPage)
				return
			}
			common.OptionMapRWMutex.RLock()
			title := operation_setting.EffectiveSiteTitle(common.SystemName)
			description := strings.TrimSpace(operation_setting.GetGeneralSetting().SiteDescription)
			common.OptionMapRWMutex.RUnlock()

			pageMutex.Lock()
			if cached.page == nil || cached.title != title || cached.description != description {
				escapedTitle := html.EscapeString(title)
				page := bytes.Replace(assets.IndexPage, []byte(`content="Unified AI API gateway and admin dashboard."`), []byte(`content="`+html.EscapeString(description)+`"`), 1)
				page = bytes.Replace(page, []byte("<title>New API</title>"), []byte("<title>"+escapedTitle+"</title>"), 1)
				page = bytes.Replace(page, []byte(`<meta name="title" content="New API"`), []byte(`<meta name="title" content="`+escapedTitle+`"`), 1)
				cached = cachedPage{title: title, description: description, page: page}
			}
			page := cached.page
			pageMutex.Unlock()
			c.Data(http.StatusOK, "text/html; charset=utf-8", page)
		},
	)
}
