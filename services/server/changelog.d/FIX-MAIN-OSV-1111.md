### Security

- Go toolchain 1.27.3 to 1.27.4 (`toolchain` of `go.mod`, the builder image
  `golang:1.27.4-trixie` of the server and of `bothy-postgres`): the standard library advisory
  GO-2026-6859 in `net/http` (an HTTP/1 server crash) fails the gate `osv`.
