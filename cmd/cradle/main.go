package main

import "github.com/rhajizada/cradle/internal/cli"

//nolint:gochecknoglobals // version is set at build time
var Version = "dev"

func main() {
	cli.Execute(Version)
}
