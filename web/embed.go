package web

import "embed"

// SwaggerFS is content from swagger.
//
//go:embed swagger
var SwaggerFS embed.FS
