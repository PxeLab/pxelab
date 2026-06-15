package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed webdist/*
var spaFS embed.FS

func spaHandler() http.Handler {
	subFS, _ := fs.Sub(spaFS, "webdist")
	return http.FileServer(http.FS(subFS))
}
