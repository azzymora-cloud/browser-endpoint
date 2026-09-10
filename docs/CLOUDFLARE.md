# Cloudflare Tunnel and Access

Named tunnel + a domain on Cloudflare is the only stable way in. The PC makes an **outbound** HTTPS connection. Cloudflare never needs an inbound port on your router.

## Named tunnel (required)

1. Open [Zero Trust](https://one.dash.cloudflare.com/) while signed into the same account that owns the domain.
2. **Networks** → **Tunnels** (Tunnels & Mesh) → **Create a tunnel**.
3. Connector: **Cloudflared**. Name: `thinkcentre` (or any name you will remember).
4. Cloudflare shows an install command like:

   ```text
   cloudflared.exe service install eyJ...
   ```

   Copy only the **`eyJ…` token**. On this project you do **not** run that command. Docker Compose runs `cloudflared` with `--token`.

5. **Published application routes** → add a route:

   | Field | Value |
   |---|---|
   | Subdomain | e.g. `desktop` |
   | Domain | pick the zone from the **dropdown** |
   | Full hostname | must show `desktop.yourdomain.com` |
   | Type | **HTTP** |
   | URL | `localhost:8080` |

   `cloudflared` shares the Guacamole container network, so the origin is `localhost:8080`, not a Compose service name.

6. Optional manage hostname:

   | Field | Value |
   |---|---|
   | Subdomain | `manage` |
   | Domain | same zone |
   | Type | HTTP |
   | URL | `host.docker.internal:18765` |

7. On the Windows PC, from the repo:

   ```powershell
   .\scripts\setup-tunnel.ps1
   docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
   ```

8. Wait until Zero Trust shows the connector **Healthy**. Then open `https://desktop.yourdomain.com`.

### Full hostname gotcha

If **Full hostname** stays `yourdomain.com` after you typed a subdomain, the Domain dropdown did not bind the zone. Open Domain, **click the zone in the list**, then confirm Full hostname is `subdomain.yourdomain.com` before Save. Saving with only the apex would publish the root domain — do not do that.

### DNS record

Saving a published route should create a **proxied CNAME**:

```text
desktop  CNAME  <tunnel-id>.cfargotunnel.com    Proxied
```

If `nslookup desktop.yourdomain.com 1.1.1.1` says **Non-existent domain**, add that CNAME yourself:

Cloudflare dashboard → the domain → **DNS** → **Add record** → CNAME, name `desktop`, target `<tunnel-id>.cfargotunnel.com`, proxy **on**.

The tunnel id is on the tunnel Overview page (a UUID). After a brand-new record, **1.1.1.1 can cache NXDOMAIN for 20–30 minutes**. `nslookup desktop.yourdomain.com 8.8.8.8` often works sooner.

### Do not install the Windows service

`docker-compose.tunnel.yml` already runs:

```text
cloudflared tunnel --no-autoupdate run --token <token>
```

A second Windows service with the same token fights the Docker connector.

## Cloudflare Access (email PIN)

Guacamole’s login is one gate. Access is a second gate **before** anyone sees Guacamole.

1. Zero Trust → **Access** → **Applications** → **Add an application** → **Self-hosted**.
2. Application name: e.g. `Desktop`.
3. Public hostname: the same hostname as the tunnel (`desktop.yourdomain.com`).
4. Identity: **One-time PIN** (email). This is what the Cloudflare **Free** plan supports. SMS / phone MFA is not on Free.
5. Policy: **Allow**, include **Emails** → your address (and any friend who should get in).
6. Session duration: 24 hours is reasonable.
7. Repeat for `manage.yourdomain.com` if you published it.

Visitors enter their email, get a PIN, then see Guacamole.

To add another person later: edit the Access policy and add their email.

Do not put Access on the Cloudflare dashboard login itself.

## Optional: trycloudflare quick test

For a one-off check before you have a domain:

```powershell
.\scripts\setup-tunnel.ps1 -QuickTest
```

The printed `https://*.trycloudflare.com` URL changes every restart. Do not use it as the daily endpoint.

## Locally-managed config (optional)

Most people should use the **token**. `cloudflare/config.yml.example` is only if you prefer `cloudflared tunnel login` + a credentials JSON. Keep `credentials.json` out of git (already gitignored).
