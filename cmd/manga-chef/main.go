// Command manga-chef is a source-agnostic manga downloader and converter.
// All logic lives in internal/cli — main.go is intentionally minimal.
package main

import "github.com/ducminhgd/manga-chef/internal/cli"

func main() {
	cli.Execute()
}
