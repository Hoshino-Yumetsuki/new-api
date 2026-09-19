package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebShellFollowsLiveSiteSettings(t *testing.T) {
	const shell = `<!doctype html><html><head><title>New API</title><meta name="title" content="New API" /><meta
      name="description"
      content="Unified AI API gateway and admin dashboard."
    /><!--umami--></head><body><div id="root"></div></body></html>`
	settings := operation_setting.GetGeneralSetting()
	oldDescription, oldName, oldOptions := settings.SiteDescription, common.SystemName, common.OptionMap
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		settings.SiteDescription, common.SystemName, common.OptionMap = oldDescription, oldName, oldOptions
		common.OptionMapRWMutex.Unlock()
	})
	common.OptionMapRWMutex.Lock()
	settings.SiteDescription = ""
	common.SystemName = "New API"
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	engine := gin.New()
	engine.GET("/api/status", controller.GetStatus)
	SetWebRouter(engine, WebAssets{BuildFS: embed.FS{}, IndexPage: []byte(shell)}, func(c *gin.Context) { c.Next() })
	request := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}
	check := func(titleHTML, titleRaw, name, descriptionHTML, descriptionRaw string) {
		t.Helper()
		page := request("/")
		require.Equal(t, http.StatusOK, page.Code)
		assert.Contains(t, page.Body.String(), "<title>"+titleHTML+"</title>")
		assert.Contains(t, page.Body.String(), `<meta name="title" content="`+titleHTML+`" />`)
		assert.Regexp(t, `<meta\s+name="description"\s+content="`+regexp.QuoteMeta(descriptionHTML)+`"\s*/>`, page.Body.String())
		assert.Equal(t, "no-cache", page.Header().Get("Cache-Control"))
		status := request("/api/status")
		require.Equal(t, http.StatusOK, status.Code)
		var payload struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, common.Unmarshal(status.Body.Bytes(), &payload))
		assert.Equal(t, name, payload.Data["system_name"])
		assert.Equal(t, titleRaw, payload.Data["site_title"])
		assert.Equal(t, descriptionRaw, payload.Data["site_description"])
	}

	check("New API", "New API", "New API", "", "")

	common.OptionMapRWMutex.Lock()
	common.SystemName = "星河"
	settings.SiteDescription = "  一站式 AI 服务  "
	common.OptionMapRWMutex.Unlock()
	check("星河 - 一站式 AI 服务", "星河 - 一站式 AI 服务", "星河", "一站式 AI 服务", "一站式 AI 服务")

	common.OptionMapRWMutex.Lock()
	common.SystemName = `ACME & <Tools>`
	settings.SiteDescription = `  Gateway " & </title><script>alert(1)</script>  `
	common.OptionMapRWMutex.Unlock()
	check("ACME &amp; &lt;Tools&gt; - Gateway &#34; &amp; &lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;", `ACME & <Tools> - Gateway " & </title><script>alert(1)</script>`, "ACME & <Tools>", "Gateway &#34; &amp; &lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;", `Gateway " & </title><script>alert(1)</script>`)
	assert.NotContains(t, request("/index.html").Body.String(), "<script>alert(1)</script>")
	assert.Equal(t, shell, request("/security").Body.String())

	common.OptionMapRWMutex.Lock()
	settings.SiteDescription = " \t "
	common.OptionMapRWMutex.Unlock()
	check("ACME &amp; &lt;Tools&gt;", "ACME & <Tools>", "ACME & <Tools>", "", "")
}
