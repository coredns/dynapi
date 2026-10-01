package dynapi

import (
	"net/netip"
	"slices"

	"github.com/miekg/dns"
)

const (
	maxIPStringLength = len("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
	maxAddresses      = 256
	maxTTL            = 1<<31 - 1
)

type recordSet struct {
	Addresses []string `json:"addresses" minItems:"1" nullable:"false" required:"true"`

	TTL uint32 `json:"ttl" maximum:"4294967295" required:"true"`
}

func (recordSet *recordSet) normalize(rrtype uint16) error {
	if len(recordSet.Addresses) == 0 || len(recordSet.Addresses) > maxAddresses ||
		recordSet.TTL > maxTTL {

		return invalidRecordSet()
	}

	for index, value := range recordSet.Addresses {
		ip, err := netip.ParseAddr(value)
		if err != nil || !matchingFamily(ip, rrtype) {
			return invalidAddress()
		}

		// Keep already-canonical strings instead of allocating replacements.
		var buffer [maxIPStringLength]byte

		canonical := ip.AppendTo(buffer[:0])
		if string(canonical) != value {
			recordSet.Addresses[index] = string(canonical)
		}
	}

	slices.Sort(recordSet.Addresses)

	recordSet.Addresses = slices.Compact(recordSet.Addresses)

	return nil
}

func matchingFamily(ip netip.Addr, rrtype uint16) bool {
	return ip.Zone() == "" && !ip.Is4In6() && (rrtype == dns.TypeA) == ip.Is4()
}
