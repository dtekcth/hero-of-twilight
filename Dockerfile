# Build
FROM golang:1.27-alpine3.24@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523 AS build
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
