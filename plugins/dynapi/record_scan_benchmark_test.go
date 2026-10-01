package dynapi

import (
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func BenchmarkAdd(b *testing.B) {
	const recordCount = 1000

	for _, test := range []struct {
		name, format, target string
	}{
		{"lowercase", "other-%d.example.org.", "host.example.org."},
		{"uppercase", "OTHER-%d.EXAMPLE.ORG.", "HOST.EXAMPLE.ORG."},
	} {
		records := make([]dns.RR, recordCount)
		for index := range records {
			records[index] = &dns.A{
				Hdr: dns.RR_Header{
					Name:   fmt.Sprintf(test.format, index),
					Rrtype: dns.TypeA,
					Ttl:    60,
				},
				A: net.ParseIP("192.0.2.1").To4(),
			}
		}

		records[0].Header().Name = test.target

		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				scan := recordScan{name: "host.example.org.", rrtype: dns.TypeA}
				if err := scan.add(records); err != nil {
					b.Fatal(err)
				}

				if len(scan.records.Addresses) != 1 {
					b.Fatal("expected one matching record")
				}
			}
		})
	}
}
