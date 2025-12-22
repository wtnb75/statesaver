package main

import (
	"embed"
)

//go:embed templates/*
var template_files embed.FS
