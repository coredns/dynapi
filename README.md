# dynapi (work-in-progress)

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

Contributions follow the [CoreDNS code of conduct](CODE_OF_CONDUCT.md) and
[Apache-2.0 license](LICENSE). Report security concerns through the
[CoreDNS security policy](SECURITY.md).

## Installation

For development, build the CoreDNS executable from this repository:

```sh
make                 # Build CoreDNS with dynapi, dynupdate, and tsig.
./coredns -plugins   # Check that all three plugins are included.
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

The development executable includes `dynupdate` and `tsig` from a pinned
CoreDNS revision. The HTTP adapter is not implemented yet.

### Configure the DNS backend

`dynupdate` serves the records and persists changes. `tsig` authenticates DNS
UPDATE requests before dynupdate applies its permission rules. TSIG authenticates
DNS clients. The future HTTP API will have its own authentication.

Create `example.org.zone` with the initial zone records:

```dns
$ORIGIN example.org.  ; Resolve relative names within this zone.
@ 60 IN SOA ns.example.org. hostmaster.example.org. 1 3600 600 86400 60 ; Define the zone and its initial serial.
@ 60 IN NS ns.example.org. ; Declare the authoritative nameserver.
ns 60 IN A 127.0.0.1 ; Point the nameserver at this local example.
```

Create a `Corefile`:

```corefile
example.org:1053 { # Serve the example zone on an unprivileged port.
    bind 127.0.0.1 # Keep this development server on loopback.
    tsig { # Authenticate signed DNS requests.
        secret update-key.example.org. {$DYNAPI_TSIG_SECRET} # Read the shared key from the environment.
        require_opcode UPDATE # Reject unsigned DNS updates.
    } # End TSIG configuration.
    dynupdate { # Serve the writable authoritative zone.
        file example.org.zone # Seed a new database with the initial zone.
        database example.org.db # Preserve committed changes across restarts.
        allow update-key.example.org. host.example.org. A AAAA # Restrict this key to one host's addresses.
    } # End writable-zone configuration.
} # End the server block.
```

Start the DNS backend:

```sh
export DYNAPI_TSIG_SECRET="$(openssl rand -base64 32)" # Generate a private shared key for this example.
./coredns -conf Corefile # Start the configured DNS server.
```

Keep the key if DNS update clients must continue using it after a restart.
The `dynapi` directive is omitted because the HTTP API is not implemented yet.

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
