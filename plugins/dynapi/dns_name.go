package dynapi

import (
	"strings"

	"github.com/miekg/dns"
)

func canonicalName(value string) (string, bool) {
	name := strings.ToLower(dns.Fqdn(value))
	if len(name) > 254 || name == "." {
		return "", false
	}

	for label := range strings.SplitSeq(strings.TrimSuffix(name, "."), ".") {
		if !validLabel(label) {
			return "", false
		}
	}

	return name, true
}

func validLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}

	for _, character := range label {
		letter := character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'

		if !letter && !digit && character != '-' && character != '_' {
			return false
		}
	}

	return true
}
