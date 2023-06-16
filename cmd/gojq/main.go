// gojq - Go implementation of jq
package main

import (
	"os"

	"github.com/momiji/gojq/cli"
)

func main() {
	os.Exit(cli.Run())
}
