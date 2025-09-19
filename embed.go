package root

import "embed"

//go:embed migrations/*
var Migrations embed.FS
