package dynapi

import (
	"testing"

	"github.com/miekg/dns"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, record string
		matches      bool
	}{
		{"uppercase owner", "HOST.EXAMPLE.ORG. 60 IN A 192.0.2.1", true},
		{"mixed case owner", "Host.Example.Org. 60 IN A 192.0.2.1", true},
		{"different owner", "other.example.org. 60 IN A 192.0.2.1", false},
		{"wildcard owner", "*.example.org. 60 IN A 192.0.2.1", false},
		{"different type", "host.example.org. 60 IN AAAA 2001:db8::1", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			record, err := dns.NewRR(test.record)
			if err != nil {
				t.Fatal(err)
			}

			scan := recordScan{name: "host.example.org.", rrtype: dns.TypeA}
			if err := scan.add([]dns.RR{record}); err != nil {
				t.Fatal(err)
			}

			matched := len(scan.records.Addresses) == 1
			if matched != test.matches || scan.count != 1 {
				t.Fatalf(
					"matched=%t count=%d; want matched=%t count=1",
					matched,
					scan.count,
					test.matches,
				)
			}
		})
	}
}
