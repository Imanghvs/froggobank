// Package docs bundles the API contract and Swagger UI with the application.
package docs

import "embed"

// Files contains the specification and the assets served at /docs/.
//
//go:embed openapi.yaml swagger/index.html swagger/swagger-initializer.js swagger/swagger-ui.css swagger/swagger-ui-bundle.js swagger/LICENSE swagger/NOTICE swagger/swagger-ui-bundle.js.LICENSE.txt
var Files embed.FS
