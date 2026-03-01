# hero-of-twilight

Link discovery of Nomad services.

Create a `config.json` with your Nomad token, Nomad URL and any static services
you want to list (see `config.json.template` for an example). Then run `go run .`
to start a local server. The server runs on port 8080.

## Configuration

The configuration file is a JSON object with the following keys:

* `token`: Your Nomad access token. Only needs `namespace:read-job` permission
* `url`: The address of your Nomad instance
* `updateInterval`: How often to query Nomad for services, specified in seconds
* `namespaces`: A list of namespaces to pick services from. Services are
  grabbed from the first namespace they are found in
* `services`: An array of objects describing static services to always include.
  See below for a description of supported keys.

## Services

Services have the following attributes:

* `name`: The name of the service (required)
* `description`: A short description of the service (required)
* `link`: A link to the service (required)

## Nomad tags

By adding tags to your Nomad jobs they can be discovered dynamically. The tags
are on the form `link-discovery.key=value`, where `key` is one of the supported
attributes for services.
