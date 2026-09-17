# Table Sync Proxy

A Cassandra / DSE / Astra DB proxy that transparently mirrors write operations
from one table to another within the same cluster — with zero application code
changes required.

Connect your application to the proxy instead of directly to Cassandra. Every
`INSERT`, `UPDATE`, or `DELETE` on a configured source table is automatically
replayed against the target table using the same values and consistency level.

---

## Features

- **Transparent write mirroring** — INSERT, UPDATE, DELETE on `tableA` are
  automatically forwarded to `tableB`
- **Multiple mappings** — configure as many source → target pairs as needed
- **Astra DB support** — use a Secure Connect Bundle for cloud databases
- **Multi-instance** — run several proxy instances on different machines with
  token-range load balancing
- **No schema duplication** — source and target tables must share the same
  schema; only the table name is rewritten in the CQL statement
- **DDL safety** — `CREATE TABLE`, `DROP TABLE`, etc. are sent to the cluster
  only once (no `AlreadyExists` errors)

---

## Quick start

### 1. Configuration

Create a `zdm-tablesync.yml` file:

```yaml
# Single cluster (self-hosted Cassandra or DSE)
cluster_contact_points: 10.0.0.1
cluster_username: cassandra
cluster_password: cassandra
cluster_port: 9042

proxy_listen_address: "0.0.0.0"
proxy_listen_port: 9042

log_level: "INFO"

table_mappings:
  - name: users-by-email
    enabled: true
    source:
      keyspace: app
      table: users_by_id
    target:
      keyspace: app
      table: users_by_email
```

For **Astra DB**, replace the contact points with a Secure Connect Bundle:

```yaml
cluster_secure_connect_bundle_path: /path/to/secure-connect-mydb.zip
cluster_username: token
cluster_password: AstraCS:...your-token...
```

### 2. Build

```bash
# Requires Go 1.22+
go build -o table-sync-proxy ./proxy
```

Cross-compile for Linux from macOS:

```bash
GOOS=linux GOARCH=amd64 go build -o table-sync-proxy-linux ./proxy
```

### 3. Run

```bash
./table-sync-proxy -config zdm-tablesync.yml
```

---

## Table mapping configuration

| Field | Required | Description |
|---|---|---|
| `name` | ✅ | Unique name for this mapping |
| `enabled` | ✅ | Set `false` to pause without removing |
| `source.keyspace` | ✅ | Source keyspace |
| `source.table` | ✅ | Source table — writes here are intercepted |
| `target.keyspace` | ✅ | Target keyspace |
| `target.table` | ✅ | Target table — mirrored writes land here |

**Rules enforced at startup:**
- Each mapping name must be unique
- Each source table may only have one enabled mapping
- Source and target must be different tables

### Multiple mappings

```yaml
table_mappings:
  - name: users-sync
    enabled: true
    source: { keyspace: app, table: users_by_id }
    target: { keyspace: app, table: users_by_email }

  - name: orders-sync
    enabled: true
    source: { keyspace: app, table: orders_by_id }
    target: { keyspace: app, table: orders_by_date }
```

---

## Multi-instance deployment

Run multiple proxy instances across machines for high availability. Point your
Cassandra driver at all proxy IPs — the driver will load-balance across them.

On each machine set the full list of proxy IPs and this machine's index:

```yaml
proxy_topology_addresses: "10.0.0.10,10.0.0.11,10.0.0.12"
proxy_topology_index: 0        # 0 on first machine, 1 on second, 2 on third
proxy_topology_num_tokens: 8   # virtual tokens per instance (default 8)
```

---

## Log levels

Set `log_level` in the config file. Recommended value for production: `WARNING`.

| Level | What you see |
|---|---|
| `DEBUG` | Every mirrored write and query inspection |
| `INFO` | Startup, connections, mapping registration |
| `WARNING` | Errors and warnings only (recommended) |
| `ERROR` | Errors only |

---

## Attribution

This project is a fork of the
[DataStax ZDM Proxy](https://github.com/datastax/zdm-proxy),
licensed under the [Apache License 2.0](LICENSE).

The table-sync feature was developed on top of the original ZDM migration proxy,
with assistance of [IBM Bob](https://www.ibm.com/products/bob).

### Changes from upstream

- Added `table_mappings` configuration for same-cluster table synchronisation
- Added `cluster_*` single-cluster config shorthand (replaces separate
  `origin_*` / `target_*` fields when both point to the same cluster)
- DDL statements (`CREATE TABLE`, `ALTER TABLE`, etc.) route to the cluster
  once only when table mappings are active
- Per-query table-sync forwarding log moved to `DEBUG` level
- Removed `columns` and `target_keys` from table mapping config — source and
  target tables are assumed to share the same schema

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
