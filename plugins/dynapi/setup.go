// Package dynapi manages CoreDNS address records through an authenticated HTTP API.
package dynapi

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

const pluginName = "dynapi"

var log = clog.NewWithPlugin(pluginName)

func init() { plugin.Register(pluginName, setup) }

func setup(controller *caddy.Controller) error {
	options, err := parse(controller)
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	cfg := dnsserver.GetConfig(controller)
	api := &dynAPI{}

	cfg.AddPlugin(func(next plugin.Handler) plugin.Handler {
		api.next = next

		return api
	})
	controller.OnStartup(func() error { return api.start(cfg, &options) })
	// Reject reload before Caddy replaces the DNS backend. A restart is required
	// until listener handoff and failed-reload recovery are implemented together.
	controller.OnRestart(func() error { return plugin.Error(pluginName, errRestartRequired) })
	controller.OnShutdown(api.stop)

	return nil
}
