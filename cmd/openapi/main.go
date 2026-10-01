// Package main exports dynapi's OpenAPI document without starting CoreDNS.
package main

import (
	"log"
	"os"

	"github.com/coredns/dynapi/plugins/dynapi"
)

func main() {
	document, err := dynapi.GenerateOpenAPI()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stdout.Write(document); err != nil {
		log.Fatal(err)
	}
}
