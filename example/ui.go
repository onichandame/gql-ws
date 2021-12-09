package main

import (
	"embed"
	"io/fs"
	"net/http"

	goutils "github.com/onichandame/go-utils"
)

//go:generate yarn
//go:generate yarn build

//go:embed dist/*
var root embed.FS

func getFS() http.FileSystem {
	dist, err := fs.Sub(root, `dist`)
	goutils.Assert(err)
	return http.FS(dist)
}
