FROM golang:1.24 AS builder
RUN CGO_ENABLED=0 go install github.com/steveyegge/beads/cmd/bd@latest

FROM ghcr.io/charmbracelet/vhs
COPY --from=builder /go/bin/bd /usr/local/bin/bd
