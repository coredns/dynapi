# ADR 0001: Use signed DNS requests for record access

Status: Accepted

Date: 2026-10-01

## Decision

Run dynapi inside CoreDNS. Read exact stored sets through signed AXFR. Replace
and delete sets atomically through signed DNS UPDATE over loopback TCP.
Dynupdate stores the zone and enforces write permissions. HTTP clients use a
bearer token. DNS requests use a separate TSIG key.

AXFR reads stored records without expanding wildcards or following CNAMEs. PUT
checks for a conflicting CNAME in the same transaction. Reads follow transfer
permissions, which are separate from write permissions.

Use `net/http` and generate OpenAPI from shared operation definitions and Go
models. The [backend](../../plugins/dynapi/backend.go) sends the DNS requests.

## Rejected

- A separate dynapi record store would duplicate persistence, permissions, and
  authoritative DNS behavior.
- Direct calls into dynupdate would require an upstream interface with defined
  transactions, permissions, and lifecycle rules.
- Ordinary DNS queries cannot reliably return exact stored sets because they
  can expand wildcards or return aliases.
- A web framework adds machinery that three HTTP operations do not need.

## Future

GET scans the whole zone. Reads and writes reuse an exclusive TCP connection
from a pool bounded by `max_requests`, which defaults to 32. Failed connections
are discarded. Sustained load tests must guide further tuning.

Conditional writes and safe Corefile reloads remain open. Dynapi does not retry
writes because a change may commit before its response is lost.

[ADR 0002](0002-add-a-coredns-record-management-interface.md) outlines an upstream
management interface that could replace the DNS bridge and dynupdate dependency.
