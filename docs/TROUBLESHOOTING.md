# Troubleshooting

## `This site can’t be reached` / `DNS_PROBE_POSSIBLE`

The public hostname has no DNS record, or a resolver cached **NXDOMAIN** from before you created it.

1. Cloudflare dashboard → domain → **DNS**. You need a **proxied CNAME** `desktop` (or whatever subdomain you chose) → `<tunnel-id>.cfargotunnel.com`.
2. Zero Trust → Tunnels → your tunnel → **Published application routes**. There should be a row for that hostname → `http://localhost:8080`.
3. `nslookup desktop.yourdomain.com 8.8.8.8` — if this works but `1.1.1.1` does not, wait for negative cache (often 20–30 minutes) or try a phone that is not on this PC’s DNS.
4. Connector must be **Healthy**. If it is Inactive, start the overlay:

   ```powershell
   docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
   docker logs thinkcentre-cloudflared-1 --tail 40
   ```

## http://127.0.0.1:8080 connection refused

Docker Desktop is stopped. Start it, wait until it is running, then:

```powershell
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

Enable **Start Docker Desktop when you log in**.

## Tunnel connector stays Inactive

- `CLOUDFLARE_TUNNEL_TOKEN` in `.env` is empty or truncated. Re-run `.\scripts\setup-tunnel.ps1` with the `eyJ…` token.
- You installed the Windows `cloudflared` service as well. Uninstall that service; only Docker should run the connector.
- Token was refreshed in the dashboard (Refresh token). Paste the new token and recreate the container.

## Guacamole login works, RDP says invalid credentials

The RDP password is the **Windows** account password, not `guacadmin`.

On Windows Pro, connecting RDP to the **same** PC you are sitting at can disconnect the local session (“Disconnected by other connection”). Test from a phone or another computer.

## Cannot change `guacadmin` on the Users page

Use **Settings → Preferences**, not Settings → Users → guacadmin. The Users page returns “use the password update endpoint”.

## Full hostname in the tunnel wizard is missing the subdomain

Pick the domain from the **dropdown list**, do not only type it. Confirm Full hostname is `subdomain.yourdomain.com` before Save.

## Agent page at :18765 is empty / 404

Install the service from the clone that contains `releases/thinkcentre-agent.exe`:

```powershell
.\scripts\install-service.ps1
```

Token is in `%ProgramData%\ThinkCentreEndpoint\config.json`. The connection log is `%ProgramData%\ThinkCentreEndpoint\logs\connections.jsonl`.

## Wake-on-LAN does nothing

- BIOS: Wake on LAN enabled, After Power Loss = Power On
- Device Manager → NIC → Power Management → allow the device to wake the computer
- A magic packet sent **from this PC** only helps if it is still on the LAN (sleep). If it is fully off, send the packet from a phone on home Wi‑Fi.

## `docker compose` cannot find the token

`.env` must live in the repo folder you run Compose from, and must contain `CLOUDFLARE_TUNNEL_TOKEN=eyJ…` with no quotes or extra spaces.
