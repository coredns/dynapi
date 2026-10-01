package dynapi

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin"
)

const (
	defaultAddress     = "127.0.0.1:8080"
	defaultMaxRequests = 32
	minTokenLength     = 32
	minSecretBytes     = 16
	maxPort            = 65535
)

type config struct {
	address     string
	token       string
	identity    string
	secret      string
	upstream    string
	maxRequests int
}

func (config *config) set(name, value string) error {
	switch name {
	case "token":
		config.token = value
	case "token_env":
		config.token = os.Getenv(value)
	case "identity":
		config.identity = value
	case "secret":
		config.secret = value
	case "secret_env":
		config.secret = os.Getenv(value)
	case "upstream":
		config.upstream = value
	case "max_requests":
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse max_requests: %w", err)
		}

		config.maxRequests = number
	default:
		return fmt.Errorf("%w: %q", errUnknownProperty, name)
	}

	return nil
}

func (config *config) validate(seen map[string]bool) error {
	if config.maxRequests < 1 {
		return errMaxRequestsConfiguration
	}

	err := config.validateCredentials(seen)
	if err != nil {
		return err
	}

	identity, valid := canonicalName(config.identity)
	if !seen["identity"] || !valid {
		return errIdentityConfiguration
	}

	config.identity = identity

	if !seen["upstream"] {
		return errUpstreamRequired
	}

	if err := loopbackAddress(config.address); err != nil {
		return err
	}

	return loopbackAddress(config.upstream)
}

func loopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errLoopbackAddress
	}

	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() || ip.Zone() != "" {
		return errLoopbackAddress
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > maxPort {
		return errInvalidPort
	}

	return nil
}

func parse(controller *caddy.Controller) (config, error) {
	var options config

	options.address = defaultAddress
	options.maxRequests = defaultMaxRequests

	if !controller.Next() {
		return options, controller.ArgErr()
	}

	args := controller.RemainingArgs()
	if len(args) > 1 {
		return options, controller.ArgErr()
	}

	if len(args) == 1 {
		options.address = args[0]
	}

	seen := map[string]bool{}

	for controller.NextBlock() {
		name := controller.Val()
		args := controller.RemainingArgs()

		if seen[name] || len(args) != 1 {
			return options, controller.Err(
				"each property requires one value and may appear only once",
			)
		}

		seen[name] = true

		err := options.set(name, args[0])
		if err != nil {
			return options, err
		}
	}

	if controller.Next() {
		return options, plugin.ErrOnce
	}

	err := options.validate(seen)

	return options, err
}

func (config *config) validateCredentials(seen map[string]bool) error {
	if seen["token"] == seen["token_env"] || len(config.token) < minTokenLength ||
		strings.ContainsAny(config.token, " \t\r\n") {

		return errTokenConfiguration
	}

	if seen["secret"] == seen["secret_env"] {
		return errSecretConfiguration
	}

	secret, err := base64.StdEncoding.DecodeString(config.secret)
	if err != nil || len(secret) < minSecretBytes {
		return errInvalidSecret
	}

	return nil
}
