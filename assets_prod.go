//go:build assets

package main

import "embed"

//go:embed assets/*
var assets embed.FS

//go:embed index.html
var indexHTML embed.FS

//go:embed pal-conf.html
var palConfHTML embed.FS

//go:embed map/*
var mapTiles embed.FS

const (
	assetsRoot      = "assets"
	indexHTMLPath   = "index.html"
	palConfHTMLPath = "pal-conf.html"
	mapRoot         = "map"
)
