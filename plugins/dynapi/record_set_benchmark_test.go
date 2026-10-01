package dynapi

import (
	"fmt"
	"slices"
	"testing"

	"github.com/miekg/dns"
)

func BenchmarkNormalize(b *testing.B) {
	cases := []struct {
		name, format string
		size         int
		rrtype       uint16
	}{
		{"A/1", "192.0.2.%d", 1, dns.TypeA},
		{"A/256", "192.0.2.%d", maxAddresses, dns.TypeA},
		{"AAAA/1", "2001:db8::%x", 1, dns.TypeAAAA},
		{"AAAA/256", "2001:db8::%x", maxAddresses, dns.TypeAAAA},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			addresses := make([]string, test.size)
			for index := range addresses {
				addresses[index] = fmt.Sprintf(test.format, test.size-index-1)
			}

			b.ReportAllocs()

			for b.Loop() {
				set := recordSet{TTL: 60, Addresses: slices.Clone(addresses)}
				if err := set.normalize(test.rrtype); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
