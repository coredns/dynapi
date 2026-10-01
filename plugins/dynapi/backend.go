package dynapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/miekg/dns"
)

const (
	tsigFudge              = 300
	poolIdleTimeoutDivisor = 2
	defaultDNSMaxQueries   = 128
	defaultDNSIdleTimeout  = 10 * time.Second
)

type backend struct {
	client  *dns.Client
	pool    *connectionPool
	zone    string
	address string
	key     string
}

func newBackend(cfg *dnsserver.Config, options *config) *backend {
	client := &dns.Client{
		Net: "tcp", Timeout: requestTimeout,
		TsigSecret: map[string]string{options.identity: options.secret},
	}
	idleTimeout := cfg.IdleTimeout

	if idleTimeout == 0 {
		idleTimeout = defaultDNSIdleTimeout
	}

	maxQueries := -1
	if cfg.MaxTCPQueries != nil {
		maxQueries = *cfg.MaxTCPQueries
	}

	if maxQueries == 0 {
		maxQueries = defaultDNSMaxQueries
	}

	return &backend{
		client:  client,
		zone:    cfg.Zone,
		address: options.upstream,
		key:     options.identity,
		// Retire idle connections before CoreDNS's own idle deadline.
		pool: newConnectionPool(
			client,
			options.upstream,
			options.maxRequests,
			idleTimeout/poolIdleTimeoutDivisor,
			maxQueries,
		),
	}
}

func (backend *backend) read(ctx context.Context, name string, rrtype uint16) (recordSet, error) {
	connection, err := backend.pool.acquire(ctx)
	if err != nil {
		return recordSet{}, err
	}

	stop := context.AfterFunc(ctx, func() { _ = connection.Conn.Close() })

	defer func() {
		stopped := stop()

		connection.reusable = connection.reusable && stopped && ctx.Err() == nil
		backend.pool.release(connection)
	}()

	err = connection.SetWriteDeadline(time.Now().Add(requestTimeout))
	if err != nil {
		return recordSet{}, fmt.Errorf("set transfer deadline: %w", err)
	}

	message := new(dns.Msg)
	message.SetAxfr(backend.zone)
	backend.sign(message)

	transfer := &dns.Transfer{
		Conn:        &dns.Conn{Conn: connection},
		TsigSecret:  backend.client.TsigSecret,
		ReadTimeout: requestTimeout,
	}

	envelopes, err := transfer.In(message, backend.address)
	if err != nil {
		return recordSet{}, fmt.Errorf("start zone transfer: %w", err)
	}

	var scan recordScan

	scan.name, scan.rrtype = name, rrtype

	if err := consumeTransfer(ctx, connection.Conn, envelopes, &scan); err != nil {
		return recordSet{}, err
	}

	connection.reusable = true

	if len(scan.records.Addresses) == 0 {
		return recordSet{}, &apiError{
			status:  http.StatusNotFound,
			code:    codeRecordSetNotFound,
			message: "record set not found",
		}
	}

	slices.Sort(scan.records.Addresses)

	return scan.records, nil
}

func (backend *backend) replace(
	ctx context.Context,
	name string,
	rrtype uint16,
	set recordSet,
) error {
	updates := make([]dns.RR, 1, len(set.Addresses)+1)

	updates[0] = emptyRecord(name, rrtype, dns.ClassANY)

	for _, address := range set.Addresses {
		header := dns.RR_Header{Name: name, Rrtype: rrtype, Class: dns.ClassINET, Ttl: set.TTL}
		if rrtype == dns.TypeA {
			updates = append(updates, &dns.A{Hdr: header, A: net.ParseIP(address).To4()})
		} else {
			updates = append(updates, &dns.AAAA{Hdr: header, AAAA: net.ParseIP(address)})
		}
	}

	// A CNAME can make RFC 2136 ignore an addition. Assert its absence in the
	// same transaction so HTTP cannot report a successful ignored replacement.
	prerequisites := []dns.RR{emptyRecord(name, dns.TypeCNAME, dns.ClassNONE)}

	return backend.update(ctx, prerequisites, updates)
}

func (backend *backend) delete(ctx context.Context, name string, rrtype uint16) error {
	return backend.update(ctx, nil, []dns.RR{emptyRecord(name, rrtype, dns.ClassANY)})
}

func (backend *backend) sign(message *dns.Msg) {
	message.SetTsig(backend.key, dns.HmacSHA256, tsigFudge, time.Now().Unix())
}

func (backend *backend) update(ctx context.Context, prerequisites, updates []dns.RR) error {
	message := new(dns.Msg)
	message.SetUpdate(backend.zone)

	message.Answer, message.Ns = prerequisites, updates
	backend.sign(message)

	connection, err := backend.pool.acquire(ctx)
	if err != nil {
		return err
	}

	stop := context.AfterFunc(ctx, func() { _ = connection.Conn.Close() })

	defer func() {
		stopped := stop()

		connection.reusable = connection.reusable && stopped && ctx.Err() == nil
		backend.pool.release(connection)
	}()

	// A fresh DNS wrapper resets the TSIG request MAC between transactions.
	response, _, err := backend.client.ExchangeWithConnContext(
		ctx,
		message,
		&dns.Conn{Conn: connection},
	)
	if err != nil {
		return fmt.Errorf("exchange DNS update: %w", err)
	}

	if response.IsTsig() == nil {
		return errUnsignedResponse
	}

	connection.reusable = true

	return updateError(response.Rcode)
}

func emptyRecord(name string, rrtype, class uint16) dns.RR {
	return &dns.RFC3597{Hdr: dns.RR_Header{Name: name, Rrtype: rrtype, Class: class}}
}

func updateError(code int) error {
	switch code {
	case dns.RcodeSuccess:
		return nil
	case dns.RcodeYXRrset, dns.RcodeYXDomain, dns.RcodeNXRrset, dns.RcodeNameError:
		return &apiError{
			status:  http.StatusConflict,
			code:    codeRecordConflict,
			message: "record prerequisite failed",
		}
	case dns.RcodeRefused:
		return &apiError{
			status: http.StatusForbidden,
			code:   codeUpdateDenied, message: "update denied by backend permissions or limits",
		}
	default:
		return &apiError{
			status:  http.StatusBadGateway,
			code:    codeBackendRejected,
			message: "backend rejected the update",
		}
	}
}
