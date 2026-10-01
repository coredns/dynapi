package dynapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/miekg/dns"
)

const (
	maxTransferRecords = 10001
	maxTransferBytes   = 16 << 20
)

type recordScan struct {
	name    string
	records recordSet
	count   int
	bytes   int
	rrtype  uint16
}

func (recordScan *recordScan) add(records []dns.RR) error {
	for _, record := range records {
		recordScan.count++

		recordScan.bytes += dns.Len(record)

		if recordScan.count > maxTransferRecords || recordScan.bytes > maxTransferBytes {
			return &apiError{
				status: http.StatusBadGateway,
				code:   codeReadLimitExceeded, message: "zone transfer exceeds the API read limit",
			}
		}

		header := record.Header()
		if header.Rrtype == recordScan.rrtype &&
			strings.EqualFold(dns.Fqdn(header.Name), recordScan.name) {

			recordScan.addAddress(record)
		}
	}

	return nil
}

func (recordScan *recordScan) addAddress(record dns.RR) {
	if len(recordScan.records.Addresses) == 0 {
		recordScan.records.TTL = record.Header().Ttl
	}

	switch address := record.(type) {
	case *dns.A:
		recordScan.records.Addresses = append(recordScan.records.Addresses, address.A.String())
	case *dns.AAAA:
		recordScan.records.Addresses = append(recordScan.records.Addresses, address.AAAA.String())
	default:
		return
	}

	recordScan.records.TTL = min(recordScan.records.TTL, record.Header().Ttl)
}

func consumeTransfer(
	ctx context.Context, conn net.Conn, envelopes <-chan *dns.Envelope, scan *recordScan,
) error {
	var failure error

	// Drain after cancellation or failure so the transfer goroutine can finish.
	for envelope := range envelopes {
		if failure != nil {
			continue
		}

		switch {
		case ctx.Err() != nil:
			failure = fmt.Errorf("zone transfer canceled: %w", ctx.Err())
		case envelope.Error != nil:
			failure = fmt.Errorf("receive zone transfer: %w", envelope.Error)
		default:
			failure = scan.add(envelope.RR)
		}

		if failure != nil {
			_ = conn.Close()
		}
	}

	return failure
}
