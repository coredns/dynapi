# Starter example

Build from the repository root, then run the server from this directory so the
zone and database paths resolve here:

```sh
make
export DYNAPI_TOKEN="$(openssl rand -hex 32)" # Share this HTTP token with the client.
export DYNAPI_TSIG_SECRET="$(openssl rand -base64 32)" # Keep this DNS key on the server.
cd examples
../coredns -conf Corefile
```

In another terminal, export the same `DYNAPI_TOKEN` and run from the repository root:

```sh
go run ./examples/client
```

The annotated [Go client](client/main.go) uses only the standard library. It
handles a missing set by its stable error code, replaces `host.example.org`'s
A records, reads them back, then deletes them. Its requests have deadlines and
it does not automatically retry writes. A failed response can follow a committed
change.

The [Corefile](Corefile) limits writes to that example host and exposes HTTP on
port 8080 and DNS on port 1053, both on loopback. The database persists in
`examples/example.org.db`. Keep the credentials when restarting the server.
