# Grafana AKIPS Integration

A Grafana data source plugin for querying [AKIPS](https://www.akips.com/) network monitoring data.

## Features

- **Time series**: plot SNMP metrics over time using AKIPS `series` commands
- **Table**: display multi-value AKIPS results as a Grafana table
- **CSV**: consume raw CSV output from any AKIPS command
- Auto-complete for devices, children (interfaces/OIDs), and attributes
- Template variables: `$__timeFrom`, `$__timeTo`, `$__timeInterval`, `$__device`, `$__child`, `$__attribute`
- Supports both `api-db` and `api-msg` endpoints

## Requirements

- Grafana ≥ 12.3.0
- An AKIPS instance with API access (`api-ro` or `api-rw` user)

## Getting started

### 1. Install the plugin

Install via the Grafana plugin catalog or by placing the built plugin in your Grafana plugins directory.

### 2. Configure the data source

| Field    | Description |
|----------|-------------|
| URL      | Base URL of your AKIPS instance, e.g. `https://akips.example.com` |
| Username | AKIPS API user, typically `api-ro` |
| Password | Password for the API user (stored securely) |
| Skip TLS | Disable TLS certificate verification (not recommended for production) |

> **Security note:** The AKIPS HTTP API does not support standard Basic authentication. Credentials are passed as URL query parameters (`username` and `password`) in every request made from the Grafana backend to AKIPS. Ensure your AKIPS instance is reachable only over HTTPS and from trusted hosts to prevent credentials from being exposed in server logs or network traffic.

### 3. Write a query

Select an **API Endpoint** (`api-db` or `api-msg`), choose a **Query type**, then optionally pick a Device / Child / Attribute from the dropdowns. These populate template variables in the query string.

**Example — time series for interface traffic:**

```
series interval avg $__timeInterval time "from $__timeFrom to $__timeTo" * "$__device" "$__child" ifInOctets
```

**Example — table of all devices:**

```
mlist device *
```

## Building from source

### Backend

```bash
mage -v
```

### Frontend

```bash
bun install
bun run build
```

### Development mode

```bash
bun run dev          # webpack watch
bun run server       # Docker Compose with Grafana
```

### Tests

```bash
bun run test:ci      # Jest unit tests
bun run e2e          # Playwright end-to-end tests (requires `bun run server` first)
```

## Contributing

Pull requests are welcome. Please open an issue first to discuss significant changes.

## License

Apache 2.0 — see [LICENSE](LICENSE).
