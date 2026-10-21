# Build
FROM golang:1.27-alpine3.24 AS build
WORKDIR /hero-of-twilight

# Copy sources
COPY go.mod main.go .
COPY static/ static/
COPY templates/ templates/

# Compile
RUN go build -o server .

# Run
FROM alpine:3.24
WORKDIR /hero-of-twilight

# Copy binary and static files
COPY --from=build /hero-of-twilight/server /hero-of-twilight/server

CMD ["/hero-of-twilight/server"]
