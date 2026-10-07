package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Imanghvs/froggobank/docs"
)

func registerDocumentation(router *gin.Engine) {
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/docs/")
	})

	for route, file := range map[string]string{
		"/docs/":                                 "swagger/index.html",
		"/docs/openapi.yaml":                     "openapi.yaml",
		"/docs/swagger-ui.css":                   "swagger/swagger-ui.css",
		"/docs/swagger-ui-bundle.js":             "swagger/swagger-ui-bundle.js",
		"/docs/swagger-initializer.js":           "swagger/swagger-initializer.js",
		"/docs/LICENSE":                          "swagger/LICENSE",
		"/docs/NOTICE":                           "swagger/NOTICE",
		"/docs/swagger-ui-bundle.js.LICENSE.txt": "swagger/swagger-ui-bundle.js.LICENSE.txt",
	} {
		router.GET(route, func(c *gin.Context) {
			if file == "openapi.yaml" {
				c.Header("Content-Type", "application/yaml")
			}
			http.ServeFileFS(c.Writer, c.Request, docs.Files, file)
		})
	}
}
