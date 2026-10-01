// Package main builds CoreDNS with the dynapi plugin.
package main

import (
	"slices"

	"github.com/coredns/coredns/core/dnsserver"
	_ "github.com/coredns/coredns/core/plugin"
	"github.com/coredns/coredns/coremain"

	_ "github.com/coredns/dynapi/plugins/dynapi"
)

func init() {
	// Match plugin.cfg.yaml when building the development executable directly.
	for i, directive := range dnsserver.Directives {
		if directive == "acl" {
			dnsserver.Directives = slices.Insert(dnsserver.Directives, i+1, "dynapi")

			return
		}
	}

	panic("cannot register dynapi: CoreDNS has no acl directive")
}

func main() {
	coremain.Run()
}
