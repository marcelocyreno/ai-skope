// Command aiss is the AI Skope Server: a local daemon that drives the coding
// agents installed on this machine and reads the folders the user allowed, so
// the AI Skope browser extension can answer questions about a web page or a
// local file.
package main

import (
	"os"

	"github.com/ai-skope/aiss/internal/cli"
)

func main() { os.Exit(cli.Main(os.Args[1:])) }
