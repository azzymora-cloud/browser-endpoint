# ThinkCentre connection template

Guacamole is seeded on first Postgres boot with two connections. Use the one that matches the Windows edition:

| Connection | When to use | Host | Port |
|---|---|---|---|
| ThinkCentre (RDP) | Windows Pro, Enterprise, Education | `host.docker.internal` | 3389 |
| ThinkCentre (VNC) | Windows Home (no inbound RDP server) | `host.docker.internal` | 5900 |

Neither connection stores a Windows or VNC password. The browser prompts when you click the connection.

If you created the database before these seeds existed, add the same settings in Guacamole: **Settings → Connections → New Connection**.

`host.docker.internal` is how Docker Desktop on Windows reaches the ThinkCentre. On Linux, Compose sets `extra_hosts: host.docker.internal:host-gateway`.
