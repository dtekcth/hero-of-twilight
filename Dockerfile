# Build
FROM golang:1.27-alpine3.24@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
WORKDIR /hero-of-twilight

# Copy sources
COPY go.mod main.go .
COPY static/ static/
COPY templates/ templates/

# Compile
RUN go build -o server .

# Run
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
WORKDIR /hero-of-twilight

# Copy binary and static files
COPY --from=build /hero-of-twilight/server /hero-of-twilight/server

CMD ["/hero-of-twilight/server"]
