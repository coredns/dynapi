package dynapi

import (
	"strings"
	"testing"

	"github.com/coredns/caddy"
)

//nolint:funlen,maintidx // Keep the cases local. Fixture data inflates size metrics.
func TestParse(t *testing.T) {
	const (
		token  = "abcdefghijklmnopqrstuvwxyz123456"
		secret = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4"
	)

	t.Setenv("DYNAPI_TEST_TOKEN", token)
	t.Setenv("DYNAPI_TEST_SECRET", secret)

	valid := `dynapi 127.0.0.1:8080 {
 token_env DYNAPI_TEST_TOKEN
 identity update-key.example.org.
 secret_env DYNAPI_TEST_SECRET
 upstream 127.0.0.1:1053
}
`
	cases := []struct {
		name, input, address string
		maxRequests          int
		invalid              bool
	}{
		{
			name:        "environment credentials",
			input:       valid,
			address:     "127.0.0.1:8080",
			maxRequests: defaultMaxRequests,
		},
		{
			name: "literal credentials",
			input: `dynapi {
 token ` + token + `
 identity update-key.example.org.
 secret ` + secret + `
 upstream 127.0.0.1:1053
 }`,
			address:     defaultAddress,
			maxRequests: defaultMaxRequests,
		},
		{
			name:        "one active request",
			input:       strings.Replace(valid, "upstream", "max_requests 1\n upstream", 1),
			address:     defaultAddress,
			maxRequests: 1,
		},
		{
			name:        "custom request limit",
			input:       strings.Replace(valid, "upstream", "max_requests 64\n upstream", 1),
			address:     defaultAddress,
			maxRequests: 64,
		},
		{
			name:    "zero request limit",
			input:   strings.Replace(valid, "upstream", "max_requests 0\n upstream", 1),
			invalid: true,
		},
		{
			name:    "negative request limit",
			input:   strings.Replace(valid, "upstream", "max_requests -1\n upstream", 1),
			invalid: true,
		},
		{
			name:    "non-integer request limit",
			input:   strings.Replace(valid, "upstream", "max_requests 1.5\n upstream", 1),
			invalid: true,
		},
		{
			name: "overflowing request limit",
			input: strings.Replace(
				valid,
				"upstream",
				"max_requests 99999999999999999999\n upstream",
				1,
			),
			invalid: true,
		},
		{
			name: "duplicate request limit",
			input: strings.Replace(
				valid,
				"upstream",
				"max_requests 1\n max_requests 2\n upstream",
				1,
			),
			invalid: true,
		},
		{name: "missing settings", input: "dynapi", invalid: true},
		{
			name:    "non-loopback listener",
			input:   strings.Replace(valid, "127.0.0.1:8080", "0.0.0.0:8080", 1),
			invalid: true,
		},
		{
			name:    "upstream hostname",
			input:   strings.Replace(valid, "127.0.0.1:1053", "localhost:1053", 1),
			invalid: true,
		},
		{
			name:    "zero port",
			input:   strings.Replace(valid, "127.0.0.1:8080", "127.0.0.1:0", 1),
			invalid: true,
		},
		{
			name:    "short token",
			input:   strings.Replace(valid, "token_env DYNAPI_TEST_TOKEN", "token short", 1),
			invalid: true,
		},
		{
			name: "invalid secret",
			input: strings.Replace(
				valid,
				"secret_env DYNAPI_TEST_SECRET",
				"secret invalid-base64",
				1,
			),
			invalid: true,
		},
		{
			name: "wildcard identity",
			input: strings.Replace(
				valid,
				"identity update-key.example.org.",
				"identity *.example.org.",
				1,
			),
			invalid: true,
		},
		{
			name: "duplicate token",
			input: strings.Replace(
				valid,
				"token_env DYNAPI_TEST_TOKEN",
				`token_env DYNAPI_TEST_TOKEN
 token_env DYNAPI_TEST_TOKEN`,
				1,
			),
			invalid: true,
		},
		{
			name: "conflicting tokens",
			input: strings.Replace(
				valid,
				"token_env DYNAPI_TEST_TOKEN",
				`token_env DYNAPI_TEST_TOKEN
 token `+token,
				1,
			),
			invalid: true,
		},
		{
			name: "conflicting secrets",
			input: strings.Replace(
				valid,
				"secret_env DYNAPI_TEST_SECRET",
				`secret_env DYNAPI_TEST_SECRET
 secret `+secret,
				1,
			),
			invalid: true,
		},
		{
			name:    "unknown property",
			input:   strings.Replace(valid, "upstream 127.0.0.1:1053", "unknown value", 1),
			invalid: true,
		},
		{name: "duplicate directive", input: valid + valid, invalid: true},
	}

	//nolint:paralleltest // These cases share process-wide environment settings.
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := parse(caddy.NewTestController("dns", test.input))
			if (err != nil) != test.invalid {
				t.Fatalf("parse error=%v; want invalid=%t", err, test.invalid)
			}

			if test.invalid {
				return
			}

			want := config{
				address: test.address, token: token, identity: "update-key.example.org.",
				secret: secret, upstream: "127.0.0.1:1053", maxRequests: test.maxRequests,
			}
			if got != want {
				t.Fatal("parsed configuration does not match the supplied settings")
			}
		})
	}
}
