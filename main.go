package main

import "github.com/rhajizada/cradle/internal/cli"

var Version = "dev"

func main() {
	cli.Execute(Version)
}
