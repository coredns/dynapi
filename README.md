# dynapi

## Name

*dynapi* - manage CoreDNS address records through an HTTP API.

## Description

*dynapi* is an external CoreDNS plugin under development. The planned API lets
applications read, replace, and delete A and AAAA record sets using JSON over
HTTP. The design builds on *dynupdate*, which owns DNS answers, record
validation, persistence, and update transactions.

This repository currently contains the plugin structure and a development
CoreDNS executable. The HTTP API is not implemented. Enabling `dynapi` returns
an explicit startup error.

[CONTRIBUTING.md](CONTRIBUTING.md) explains local builds and checks.

Contributions follow the [CoreDNS code of conduct](CODE_OF_CONDUCT.md) and
[Apache-2.0 license](LICENSE). Report security concerns through the
[CoreDNS security policy](SECURITY.md).

## Installation

For development, build the CoreDNS executable from this repository:

```sh
make
./coredns -plugins
```

To include the plugin in another CoreDNS build, add this entry to `plugin.cfg`
after `acl`:

```text
dynapi:github.com/coredns/dynapi/plugins/dynapi
```

Then resolve a reviewed dynapi revision and rebuild CoreDNS:

```sh
go get github.com/coredns/dynapi/plugins/dynapi@REVISION
go generate coredns.go
go build -o coredns .
```

Replace `REVISION` with a commit or release tag. There is no dynapi release
yet. `plugin.cfg.yaml` records the intended placement for external build
tooling. The root executable sets the same placement in Go.

The current CoreDNS v1.14.7 dependency validates packaging only. It does not
include `dynupdate`. Runtime implementation requires a pinned CoreDNS revision
that provides the selected backend interface or the DNS UPDATE bridge.

## Syntax

The Corefile syntax is still being designed. The only recognized directive
in this scaffold is `dynapi`, and it refuses startup. Backend configuration
and a complete example will follow when the API is implemented.

## Examples

The planned HTTP resource is:

```text
/v1/zones/{zone}/records/{name}/{type}
```

A PUT request replaces the complete record set. Its proposed body is:

```json
{"ttl":60,"addresses":["192.0.2.10","192.0.2.11"]}
```

GET reads the exact stored record set. DELETE removes it. The initial types
are A and AAAA. TTL controls DNS caching and does not expire or delete records.
