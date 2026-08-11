FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/generate-invoices ./cmd/generate-invoices

FROM alpine:3.20
RUN adduser -D -u 10001 conduit
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/generate-invoices /usr/local/bin/generate-invoices
USER conduit
EXPOSE 8004
ENTRYPOINT ["/usr/local/bin/server"]
