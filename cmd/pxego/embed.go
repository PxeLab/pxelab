package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist/*
var spaFS embed.FS

func spaHandler() http.Handler {
	subFS, _ := fs.Sub(spaFS, "web/dist")
	return http.FileServer(http.FS(subFS))
}
