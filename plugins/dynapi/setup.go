// Package dynapi registers an HTTP record-management plugin for CoreDNS.
package dynapi

import (
	"errors"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin"
)

const pluginName = "dynapi"

func init() { plugin.Register(pluginName, setup) }

func setup(_ *caddy.Controller) error {
	// Reject configuration until HTTP requests can reach the update backend.
	return plugin.Error(pluginName, errors.New("HTTP record management is not implemented yet"))
}
