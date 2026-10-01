//go:build integration

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type httpFixture struct {
	t         *testing.T
	client    *http.Client
	binary    string
	directory string
	baseURL   string
	token     string
	secret    string
	dnsPort   int
	httpPort  int
	direct    bool
}

func newHTTPFixture(t *testing.T, direct bool) *httpFixture {
	t.Helper()

	fixture := &httpFixture{t: t, direct: direct}

	fixture.binary = os.Getenv("COREDNS_DYNAPI_BINARY")

	if fixture.binary == "" {
		t.Fatal("COREDNS_DYNAPI_BINARY is required. Run make integration.")
	}

	fixture.directory = t.TempDir()
	fixture.dnsPort, fixture.httpPort = unusedPort(t), unusedPort(t)

	for fixture.dnsPort == fixture.httpPort {
		fixture.httpPort = unusedPort(t)
	}

	fixture.token = strings.Repeat("a", 32)
	fixture.secret = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 24)))

	fixture.configure()

	fixture.client = &http.Client{Timeout: 10 * time.Second}
	t.Cleanup(fixture.client.CloseIdleConnections)

	fixture.baseURL = fmt.Sprintf(
		"http://127.0.0.1:%d/v1/zones/example.org/records/",
		fixture.httpPort,
	)

	return fixture
}

func (httpFixture *httpFixture) request(
	method, name, rrtype, body, auth string,
) (int, string) {
	t := httpFixture.t
	t.Helper()

	r, err := http.NewRequestWithContext(
		t.Context(),
		method,
		httpFixture.baseURL+name+"/"+rrtype,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}

	if auth != "" {
		r.Header.Set("Authorization", "Bearer "+auth)
	}

	r.Header.Set("Content-Type", "application/json")

	response, err := httpFixture.client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestBody(t, response.Body)

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	return response.StatusCode, string(bytes)
}

func (httpFixture *httpFixture) start() (*exec.Cmd, func()) {
	t := httpFixture.t
	t.Helper()

	logfile, err := os.CreateTemp(httpFixture.directory, "server-*.log")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := logfile.Close(); err != nil {
			t.Error(err)
		}
	})

	//nolint:gosec // The test runner supplies the CoreDNS executable.
	cmd := exec.CommandContext(t.Context(), httpFixture.binary, "-conf", "Corefile")

	cmd.Dir = httpFixture.directory

	cmd.Env = append(
		os.Environ(),
		"GORACE=halt_on_error=1",
		"DYNAPI_TEST_TOKEN="+httpFixture.token,
		"DYNAPI_TEST_SECRET="+httpFixture.secret,
	)
	cmd.Stdout, cmd.Stderr = logfile, logfile

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	var once sync.Once

	stop := func() {
		once.Do(func() {
			stopTestProcess(t, cmd, done)
		})
	}
	t.Cleanup(stop)

	httpFixture.waitReady(logfile, done)

	return cmd, stop
}

func (httpFixture *httpFixture) query(name string, rrtype uint16) *dns.Msg {
	return httpFixture.queryNetwork(name, rrtype, "tcp")
}

func (httpFixture *httpFixture) queryNetwork(name string, rrtype uint16, network string) *dns.Msg {
	t := httpFixture.t
	t.Helper()

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), rrtype)

	response, _, err := (&dns.Client{Net: network, Timeout: 5 * time.Second}).ExchangeContext(
		t.Context(),
		m,
		fmt.Sprintf("127.0.0.1:%d", httpFixture.dnsPort),
	)
	if err != nil {
		t.Fatal(err)
	}

	return response
}

func (httpFixture *httpFixture) checkDNS(rrtype uint16, want ...string) {
	t := httpFixture.t
	t.Helper()

	for _, network := range []string{"tcp", "udp"} {
		response := httpFixture.queryNetwork("host.example.org", rrtype, network)

		var got []string

		for _, rr := range response.Answer {
			switch rr := rr.(type) {
			case *dns.A:
				got = append(got, rr.A.String())
			case *dns.AAAA:
				got = append(got, rr.AAAA.String())
			default:
				t.Fatalf("unexpected DNS answer type: %T", rr)
			}
		}

		slices.Sort(got)
		slices.Sort(want)

		if response.Rcode != dns.RcodeSuccess || !response.Authoritative ||
			!slices.Equal(got, want) {

			t.Fatalf("DNS answer=%s; want=%v", response, want)
		}
	}
}

func (httpFixture *httpFixture) configure() {
	t := httpFixture.t
	t.Helper()

	corefile := fmt.Sprintf(`example.org:%d {
    bind 127.0.0.1
    cache 30
    dynapi 127.0.0.1:%d {
        token_env DYNAPI_TEST_TOKEN
        identity update-key.example.org.
        secret_env DYNAPI_TEST_SECRET
        upstream 127.0.0.1:%d
        max_requests 8
    }
    tsig {
        secret update-key.example.org. {$DYNAPI_TEST_SECRET}
        require_opcode UPDATE
        require AXFR
    }
    transfer {
        to 127.0.0.1
    }
    dynupdate {
        file example.org.zone
        database example.org.db
        allow update-key.example.org. host.example.org. A AAAA
    }
}
`, httpFixture.dnsPort, httpFixture.httpPort, httpFixture.dnsPort)
	if httpFixture.direct {
		corefile = strings.Replace(
			corefile,
			"token_env DYNAPI_TEST_TOKEN",
			"token "+httpFixture.token,
			1,
		)
		corefile = strings.Replace(
			corefile,
			"secret_env DYNAPI_TEST_SECRET",
			"secret "+httpFixture.secret,
			1,
		)
	}

	zone := `$ORIGIN example.org.
@ 60 IN SOA ns.example.org. hostmaster.example.org. 1 3600 600 86400 60
@ 60 IN NS ns.example.org.
ns 60 IN A 127.0.0.1
alias 60 IN CNAME ns.example.org.
*.wild 60 IN A 192.0.2.99
`

	for name, contents := range map[string]string{"Corefile": corefile, "example.org.zone": zone} {
		err := os.WriteFile(filepath.Join(httpFixture.directory, name), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func (httpFixture *httpFixture) waitReady(logfile *os.File, done chan error) {
	t := httpFixture.t
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if httpFixture.isReady() {
			return
		}

		select {
		case err := <-done:
			// Cleanup must still be able to observe process completion.
			done <- err

			logs, _ := os.ReadFile(logfile.Name())
			t.Fatalf("CoreDNS exited: %v\n%s", err, logs)
		default:
		}

		time.Sleep(20 * time.Millisecond)
	}

	logs, _ := os.ReadFile(logfile.Name())
	t.Fatalf("HTTP API did not become ready:\n%s", logs)
}

func closeTestBody(t *testing.T, body io.Closer) {
	t.Helper()

	err := body.Close()
	if err != nil {
		t.Error(err)
	}
}

func stopTestProcess(t *testing.T, cmd *exec.Cmd, done <-chan error) {
	t.Helper()

	if err := cmd.Process.Signal(
		os.Interrupt,
	); err != nil &&
		!errors.Is(err, os.ErrProcessDone) {

		t.Error(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("CoreDNS exited with an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("CoreDNS did not stop within 10 seconds")

		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}

		<-done
	}
}

func (httpFixture *httpFixture) isReady() bool {
	t := httpFixture.t
	t.Helper()

	r, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		httpFixture.baseURL+"host.example.org/A",
		http.NoBody,
	)
	r.Header.Set("Authorization", "Bearer "+httpFixture.token)

	response, err := httpFixture.client.Do(r)
	if err != nil {
		return false
	}

	_, copyErr := io.Copy(io.Discard, response.Body)

	closeErr := response.Body.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}

	if copyErr != nil {
		t.Fatal(copyErr)
	}

	return response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusOK
}

func (httpFixture *httpFixture) checkDNSTTL(rrtype uint16, ttl uint32) {
	t := httpFixture.t
	t.Helper()

	for _, network := range []string{"tcp", "udp"} {
		response := httpFixture.queryNetwork("host.example.org", rrtype, network)
		for _, record := range response.Answer {
			if record.Header().Ttl != ttl {
				t.Fatalf("%s DNS TTL=%d; want=%d", network, record.Header().Ttl, ttl)
			}
		}
	}
}
