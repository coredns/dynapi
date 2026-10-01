package dynapi

import (
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		addresses, want []string
		rrtype          uint16
	}{
		{"canonical IPv4", []string{"192.0.2.1"}, []string{"192.0.2.1"}, dns.TypeA},
		{"canonical IPv6", []string{"2001:db8::1"}, []string{"2001:db8::1"}, dns.TypeAAAA},
		{
			"expanded IPv6",
			[]string{"2001:0DB8:0000:0000:0000:0000:0000:0001"},
			[]string{"2001:db8::1"},
			dns.TypeAAAA,
		},
		{
			"longest canonical IPv6",
			[]string{"FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF"},
			[]string{"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"},
			dns.TypeAAAA,
		},
		{
			"normalize before deduplication",
			[]string{"2001:db8::2", "2001:0db8:0:0:0:0:0:1", "2001:db8::1"},
			[]string{"2001:db8::1", "2001:db8::2"},
			dns.TypeAAAA,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			set := recordSet{TTL: 60, Addresses: test.addresses}
			if err := set.normalize(test.rrtype); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(set.Addresses, test.want) || set.TTL != 60 {
				t.Fatalf("normalized set=%+v; want addresses=%v and ttl=60", set, test.want)
			}
		})
	}
}
