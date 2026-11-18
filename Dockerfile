# Build
FROM golang:1.27-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /hero-of-twilight

# Copy sources
COPY go.mod main.go .
COPY static/ static/
COPY templates/ templates/

# Compile
RUN go build -o server .

# Run
FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
WORKDIR /hero-of-twilight

# Copy binary and static files
COPY --from=build /hero-of-twilight/server /hero-of-twilight/server

CMD ["/hero-of-twilight/server"]
