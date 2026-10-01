package dynapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dynupdate"
	"github.com/miekg/dns"
)

const (
	headerTimeout  = 5 * time.Second
	readTimeout    = 10 * time.Second
	writeTimeout   = 15 * time.Second
	idleTimeout    = 30 * time.Second
	maxHeaderBytes = 8 << 10
)

type dynAPI struct {
	next    plugin.Handler
	server  *http.Server
	backend *backend
	ready   atomic.Bool
}

// Name identifies the plugin in the CoreDNS handler chain.
func (*dynAPI) Name() string { return pluginName }

// Ready reports whether the HTTP listener is accepting requests.
func (dynAPI *dynAPI) Ready() bool { return dynAPI.ready.Load() }

// ServeDNS leaves ordinary DNS requests to the configured DNS plugins.
func (dynAPI *dynAPI) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return plugin.NextOrFailure(pluginName, dynAPI.next, ctx, w, r)
}

func (dynAPI *dynAPI) serve(listener net.Listener) {
	defer dynAPI.ready.Store(false)

	err := dynAPI.server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Errorf("HTTP listener failed: %v", err)
	}
}

func (dynAPI *dynAPI) start(cfg *dnsserver.Config, options *config) error {
	if err := validateBackend(cfg, options); err != nil {
		return plugin.Error(pluginName, err)
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", options.address)
	if err != nil {
		return fmt.Errorf("listen for HTTP requests: %w", err)
	}

	dynAPI.backend = newBackend(cfg, options)

	dynAPI.server = &http.Server{
		Handler:           newHandler(cfg.Zone, options.token, dynAPI.backend, options.maxRequests),
		ReadHeaderTimeout: headerTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	dynAPI.ready.Store(true)

	go dynAPI.serve(listener)

	return nil
}

func (dynAPI *dynAPI) stop() error {
	dynAPI.ready.Store(false)

	if dynAPI.server == nil {
		return nil
	}

	defer dynAPI.backend.pool.close()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	err := dynAPI.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("drain HTTP requests: %w", errors.Join(err, dynAPI.server.Close()))
	}

	return nil
}

func validateBackend(cfg *dnsserver.Config, options *config) error {
	zone, valid := cfg.Handler("dynupdate").(*dynupdate.DynUpdate)
	if !valid || zone.Zone != cfg.Zone {
		return errDNSBackend
	}

	if cfg.Handler("tsig") == nil || cfg.Handler("transfer") == nil {
		return errBridgePlugins
	}

	host, port, err := net.SplitHostPort(options.upstream)
	if err != nil {
		return fmt.Errorf("parse DNS upstream: %w", err)
	}

	if port != cfg.Port {
		return errDNSPort
	}

	if !slices.Contains(cfg.ListenHosts, "") && !slices.Contains(cfg.ListenHosts, host) {
		return errDNSHost
	}

	return nil
}
