//go:build integration

package main

import (
	"encoding/json/v2"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestHTTPAPI(t *testing.T) {
	t.Parallel()

	for _, mode := range []struct {
		name   string
		direct bool
	}{
		{"environment", false}, {"literal", true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			checkHTTPAPI(t, mode.direct)
		})
	}
}

func checkHTTPAPI(t *testing.T, direct bool) {
	t.Helper()

	fixture := newHTTPFixture(t, direct)
	cmd, stop := fixture.start()

	checkInitialState(t, fixture)
	checkReplacements(t, fixture)
	checkConcurrentAPI(t, fixture)
	checkPermissions(t, fixture)
	checkUnsignedUpdate(t, fixture)
	checkReload(t, fixture, cmd)
	stop()

	_, stop = fixture.start()
	fixture.checkDNS(dns.TypeA, "192.0.2.12")
	fixture.checkDNS(dns.TypeAAAA, "2001:db8::10")
	checkDeletion(t, fixture)
	stop()
}

func checkInitialState(t *testing.T, fixture *httpFixture) {
	t.Helper()

	if status, _ := fixture.request(
		http.MethodPut,
		"host.example.org",
		"A",
		`{"ttl":60,"addresses":["192.0.2.10"]}`,
		"",
	); status != 401 {
		t.Fatalf("unauthorized status=%d", status)
	}

	if response := fixture.query(
		"host.example.org",
		dns.TypeA,
	); response.Rcode != dns.RcodeNameError {
		t.Fatalf("expected initial NXDOMAIN: %s", response)
	}
}

func checkReplacements(t *testing.T, fixture *httpFixture) {
	t.Helper()

	for _, set := range []struct {
		rrtype, body string
		addresses    []string
		ttl          uint32
		dnsType      uint16
	}{
		{
			"A",
			`{"ttl":60,"addresses":["192.0.2.10","192.0.2.11"]}`,
			[]string{"192.0.2.10", "192.0.2.11"},
			60, dns.TypeA,
		},
		{"A", `{"ttl":120,"addresses":["192.0.2.12"]}`, []string{"192.0.2.12"}, 120, dns.TypeA},
		{"AAAA", `{"ttl":60,"addresses":["2001:db8::10"]}`, []string{"2001:db8::10"}, 60, dns.TypeAAAA},
	} {
		status, body := fixture.request(
			http.MethodPut,
			"host.example.org",
			set.rrtype,
			set.body,
			fixture.token,
		)
		checkRecordResponse(t, status, body, set.addresses, set.ttl)

		fixture.checkDNS(set.dnsType, set.addresses...)
		fixture.checkDNSTTL(set.dnsType, set.ttl)

		status, body = fixture.request(
			http.MethodGet,
			"host.example.org",
			set.rrtype,
			"",
			fixture.token,
		)

		checkRecordResponse(t, status, body, set.addresses, set.ttl)
	}
}

func checkPermissions(t *testing.T, fixture *httpFixture) {
	t.Helper()

	for _, test := range []struct {
		method, name, body string
		status             int
	}{
		{http.MethodPut, "denied.example.org", `{"ttl":60,"addresses":["192.0.2.15"]}`, 403},
		{http.MethodPut, "alias.example.org", `{"ttl":60,"addresses":["192.0.2.15"]}`, 409},
		{http.MethodGet, "alias.example.org", "", 404},
		{http.MethodGet, "new.wild.example.org", "", 404},
		{http.MethodPut, "host.example.org", `{"ttl":60,"addresses":["2001:db8::1"]}`, 422},
	} {
		if status, body := fixture.request(
			test.method,
			test.name,
			"A",
			test.body,
			fixture.token,
		); status != test.status {
			t.Fatalf("%s %s: status=%d body=%s", test.method, test.name, status, body)
		}
	}

	if response := fixture.query("new.wild.example.org", dns.TypeA); len(response.Answer) != 1 {
		t.Fatalf("wildcard DNS query failed: %s", response)
	}
}

func checkUnsignedUpdate(t *testing.T, fixture *httpFixture) {
	t.Helper()

	unsigned := new(dns.Msg)
	unsigned.SetUpdate("example.org.")
	unsigned.Insert(
		[]dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "host.example.org.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: net.ParseIP("192.0.2.55").To4(),
			},
		},
	)

	response, _, err := (&dns.Client{Net: "tcp", Timeout: 5 * time.Second}).Exchange(
		unsigned,
		fmt.Sprintf("127.0.0.1:%d", fixture.dnsPort),
	)
	if err != nil || response.Rcode != dns.RcodeRefused {
		t.Fatalf("unsigned UPDATE response=%v error=%v", response, err)
	}

	fixture.checkDNS(dns.TypeA, "192.0.2.12")
}

func checkReload(t *testing.T, fixture *httpFixture, cmd *exec.Cmd) {
	t.Helper()

	err := cmd.Process.Signal(syscall.SIGUSR1)
	if err != nil {
		t.Fatal(err)
	}

	// A refused reload must leave the existing listener and zone usable.
	time.Sleep(100 * time.Millisecond)

	if status, body := fixture.request(
		http.MethodGet,
		"host.example.org",
		"A",
		"",
		fixture.token,
	); status != 200 {
		t.Fatalf("after refused reload: status=%d body=%s", status, body)
	}
}

func checkDeletion(t *testing.T, fixture *httpFixture) {
	t.Helper()

	for range 2 {
		if status, body := fixture.request(
			http.MethodDelete,
			"host.example.org",
			"A",
			"",
			fixture.token,
		); status != 204 {
			t.Fatalf("DELETE status=%d body=%s", status, body)
		}
	}

	if status, body := fixture.request(
		http.MethodGet,
		"host.example.org",
		"A",
		"",
		fixture.token,
	); status != 404 {
		t.Fatalf("deleted GET status=%d body=%s", status, body)
	}

	fixture.checkDNS(dns.TypeA)
	fixture.checkDNS(dns.TypeAAAA, "2001:db8::10")
}

func checkRecordResponse(t *testing.T, status int, body string, addresses []string, ttl uint32) {
	t.Helper()

	var got struct {
		Addresses []string `json:"addresses"`
		TTL       uint32   `json:"ttl"`
	}

	err := json.Unmarshal([]byte(body), &got)

	if err != nil || status != http.StatusOK || got.TTL != ttl ||
		!slices.Equal(got.Addresses, addresses) {

		t.Fatalf(
			"HTTP status=%d body=%s; want addresses=%v ttl=%d; error=%v",
			status,
			body,
			addresses,
			ttl,
			err,
		)
	}
}

func checkConcurrentAPI(t *testing.T, fixture *httpFixture) {
	t.Helper()

	const clients = 8

	t.Run("concurrent API requests", func(t *testing.T) {
		for index := range clients {
			t.Run(fmt.Sprintf("client-%d", index), func(t *testing.T) {
				t.Parallel()

				concurrent := *fixture

				concurrent.t = t

				for _, method := range []string{http.MethodPut, http.MethodGet} {
					body := ""
					if method == http.MethodPut {
						body = `{"ttl":120,"addresses":["192.0.2.12"]}`
					}

					status, response := concurrent.request(
						method,
						"host.example.org",
						"A",
						body,
						concurrent.token,
					)
					checkRecordResponse(t, status, response, []string{"192.0.2.12"}, 120)
				}
			})
		}
	})
}
