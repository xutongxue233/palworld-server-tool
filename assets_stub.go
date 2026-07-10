//go:build !assets

package main

import "embed"

//go:embed internal/embedfallback/assets/*
var assets embed.FS

//go:embed internal/embedfallback/index.html
var indexHTML embed.FS

//go:embed internal/embedfallback/pal-conf.html
var palConfHTML embed.FS

//go:embed internal/embedfallback/map/*
var mapTiles embed.FS

const (
	assetsRoot      = "internal/embedfallback/assets"
	indexHTMLPath   = "internal/embedfallback/index.html"
	palConfHTMLPath = "internal/embedfallback/pal-conf.html"
	mapRoot         = "internal/embedfallback/map"
)
