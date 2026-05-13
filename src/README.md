# Grafana AKIPS Integration

Query [AKIPS](https://www.akips.com/) network monitoring data directly in Grafana as time series, tables, or raw CSV.

## Overview

AKIPS is a high-performance SNMP network monitoring platform. This plugin connects Grafana to your AKIPS instance via its HTTP API, letting you build dashboards for device metrics, interface traffic, error counters, and any other data AKIPS collects.

## Requirements

- Grafana ≥ 12.3.0
- An AKIPS instance reachable from the Grafana server
- An AKIPS API user (`api-ro` for read-only access)

## Configuration

| Field    | Description |
|----------|-------------|
| URL      | Base URL of your AKIPS instance, e.g. `https://akips.example.com` |
| Username | AKIPS API username |
| Password | AKIPS API password (stored as a Grafana secure field) |
| Skip TLS | Bypass TLS certificate verification — use only for self-signed certs in trusted environments |

> **Security note:** The AKIPS HTTP API does not support standard Basic authentication. Credentials are passed as URL query parameters (`username` and `password`) in every request made from the Grafana backend to AKIPS. Ensure your AKIPS instance is reachable only over HTTPS and from trusted hosts to prevent credentials from being exposed in server logs or network traffic.

## Query editor

### API Endpoint

Choose `api-db` (device/attribute database) or `api-msg` (message/event data).

### Query type

| Type        | Description |
|-------------|-------------|
| Time series | Returns one Grafana series per AKIPS output line |
| Table       | Returns a flat table of Parent / Child / Attribute / Values |
| CSV         | Passes raw CSV output through as a table |

### Device / Child / Attribute dropdowns

Selecting a device, child (interface or OID), and attribute populates the `$__device`, `$__child`, and `$__attribute` template variables in your query string.

### Template variables

| Variable          | Value |
|-------------------|-------|
| `$__timeFrom`     | Query start time as a Unix timestamp |
| `$__timeTo`       | Query end time as a Unix timestamp |
| `$__timeInterval` | Grafana panel interval rounded up to the nearest 60 s |
| `$__device`       | Selected device |
| `$__child`        | Selected child |
| `$__attribute`    | Selected attribute (defaults to `*`) |

### Example queries

**Interface traffic over time:**
```
series interval avg $__timeInterval time "from $__timeFrom to $__timeTo" * "$__device" "$__child" ifInOctets
```

**All devices (table):**
```
mlist device *
```

**Recent syslog messages (api-msg):**
```
mget * * syslog
```

## License

Apache 2.0
