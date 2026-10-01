package dynapi

import "errors"

var (
	errPoolClosed               = errors.New("DNS connection pool is closed")
	errMaxRequestsConfiguration = errors.New("max_requests must be a positive integer")
	errUnknownProperty          = errors.New("unknown dynapi property")
	errTokenConfiguration       = errors.New(
		"configure token or token_env with a token of 32+ characters without whitespace",
	)
	errIdentityConfiguration = errors.New(
		"identity must be a literal dynupdate permission key name",
	)
	errSecretConfiguration = errors.New("configure secret or secret_env for TSIG signing")
	errInvalidSecret       = errors.New("TSIG secret must be base64 encoding at least 16 bytes")
	errUpstreamRequired    = errors.New("upstream is required")
	errLoopbackAddress     = errors.New("address must be a literal loopback IP and port")
	errInvalidPort         = errors.New("port must be 1..65535")
	errUnsignedResponse    = errors.New("unsigned DNS update response")
	errDNSBackend          = errors.New("dynupdate must serve the same single zone")
	errBridgePlugins       = errors.New("tsig and transfer are required for the DNS bridge")
	errDNSPort             = errors.New("upstream must use this server block's DNS port")
	errDNSHost             = errors.New("upstream must use an address bound by this server block")
	errRestartRequired     = errors.New("configuration changes require a process restart")
)
