# Build
FROM golang:1.25.4-alpine3.22 AS build
WORKDIR /hero-of-twilight

# Copy sources
COPY go.mod main.go .
COPY static/ static/

# Compile
RUN go build -o server .

# Run
FROM alpine:3.23.2
WORKDIR /hero-of-twilight

# Copy binary and static files
COPY --from=build /hero-of-twilight/server /hero-of-twilight/server

CMD /hero-of-twilight/server
