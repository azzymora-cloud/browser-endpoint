# ThinkCentre browser remote endpoint

Turn a Windows Lenovo ThinkCentre into a desktop you can open from **any browser**, including hotel, school, and work Wi‑Fi that blocks VPNs, UPnP, and apps like Parsec.

The ThinkCentre stays the computer. [Apache Guacamole](https://guacamole.apache.org/) paints its desktop in HTML5. [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) carries that page over **outbound HTTPS on port 443**. You never forward RDP, never enable UPnP, and you do not install a VPN on the laptop or phone you are borrowing.

## Why this works where Parsec and VPNs fail

Parsec, UPnP, and many VPN clients need **inbound ports**, **UDP hole punching**, or a protocol the firewall already flags.

This stack does the opposite:

1. The ThinkCentre only **dials out** to Cloudflare on 443 (same as opening a website).
2. Your browser on the restricted network does **not** run a VPN. It loads a normal `https://` hostname.
3. The desktop is relayed **inside that HTTPS session** (WebSockets over TLS), not a peer-to-peer UDP path.

Tailscale / WireGuard is **not** the way in. Networks that “block VPNs” often block those by name or UDP. Cloudflare looks like ordinary web traffic.

**Still will not work if** the client network blocks Cloudflare entirely, a proxy allows HTTPS but strips WebSockets, or you have not completed a captive portal. The home ISP only needs outbound 443.

```text
Browser on locked-down Wi-Fi
        |  HTTPS :443
        v
Cloudflare edge
        |  existing outbound tunnel
        v
cloudflared on the ThinkCentre
        v
Guacamole (localhost only)
        v
Windows desktop (RDP or VNC on the PC, not on the internet)
```

## What you need

- ThinkCentre running Windows (Home or Pro)
- [Docker Desktop for Windows](https://docs.docker.com/desktop/setup/install/windows-install/) with the WSL2 backend
- A free [Cloudflare](https://dash.cloudflare.com/) account and a **domain** on Cloudflare (required for a stable URL)
- The ThinkCentre left powered on, not sleeping

A `trycloudflare.com` quick tunnel is only for a one-off test. The URL changes every restart.

## Windows edition

| Edition | Desktop protocol | Guacamole connection |
|---|---|---|
| Pro, Enterprise, Education | Remote Desktop (RDP, port 3389) | **ThinkCentre (RDP)** |
| Home | TightVNC (port 5900) — Windows Home cannot host RDP | **ThinkCentre (VNC)** |

`scripts/setup-windows.ps1` detects the edition. RDP and VNC stay off the public internet. The firewall allows those ports only from loopback and RFC1918 (so Docker Desktop can reach `host.docker.internal`). Do **not** port-forward 3389 or 5900 on the router.

## Install on the ThinkCentre

Run PowerShell **as Administrator** from this repo:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\setup-windows.ps1
```

On Windows Home you will be asked for a TightVNC password (8 characters max).

Install Docker Desktop if the script says it is missing, then:

```powershell
copy .env.example .env
notepad .env
```

Set a strong `POSTGRES_PASSWORD`. Leave `GUACAMOLE_BIND=127.0.0.1` so Guacamole is not published on the LAN.

```powershell
docker compose up -d
```

On the ThinkCentre, open [http://127.0.0.1:8080](http://127.0.0.1:8080).

- First login: `guacadmin` / `guacadmin`
- **Change that password immediately** (Guacamole → Settings → Preferences, or Users)
- Click **ThinkCentre (RDP)** or **ThinkCentre (VNC)** and sign in with the Windows (or TightVNC) password

If the local session works, expose it:

```powershell
.\scripts\setup-tunnel.ps1
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

The script walks through creating a **named** tunnel in [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → Networks → Tunnels. Public hostname example: `pc.example.com` → `http://127.0.0.1:8080` (cloudflared runs in the same network namespace as Guacamole).

When the connector is healthy, open `https://pc.example.com` from anywhere HTTPS works — including networks that block Parsec and VPNs.

### Optional: Cloudflare Access (email PIN)

The tunnel URL is public. Guacamole’s password is one gate. Add a second gate:

1. Zero Trust → Access → Applications → Add an application → Self-hosted
2. Domain = the same hostname (`pc.example.com`)
3. Policy: include your email, identity provider **One-time PIN**

Visitors get an email code before they ever see Guacamole.

### Optional: one-off `trycloudflare.com` test

```powershell
.\scripts\setup-tunnel.ps1 -QuickTest
```

Do not use that URL as your daily endpoint.

## Repository layout

| Path | Role |
|---|---|
| `docker-compose.yml` | `guacd`, Postgres, Guacamole (localhost only) |
| `docker-compose.tunnel.yml` | `cloudflared` overlay (needs `CLOUDFLARE_TUNNEL_TOKEN`) |
| `guacamole/init/` | Official Guacamole 1.6.0 schema plus ThinkCentre connection seeds |
| `guacamole/connection-template.md` | RDP/VNC settings if you add connections by hand |
| `scripts/setup-windows.ps1` | Home vs Pro, firewall, always-on power, UPnP off |
| `scripts/setup-tunnel.ps1` | Save the named-tunnel token |
| `cloudflare/config.yml.example` | Locally-managed tunnel (token method is preferred) |
| `.env.example` | Secrets template |

`docker compose up -d` starts Guacamole only. Add the tunnel overlay when you have a token:

```powershell
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

## Keep the box reachable

- Leave the ThinkCentre powered; the setup script turns off sleep and hibernate on AC
- Do not put the NIC to sleep in Device Manager → network adapter → Power Management
- UPnP is disabled on purpose. This design never uses it

## Backup

Guacamole users and connections live in the `postgres_data` Docker volume. Tunnel credentials live in `.env` (`CLOUDFLARE_TUNNEL_TOKEN`).

```powershell
docker compose pause
docker run --rm -v thinkcentre_postgres_data:/data -v ${PWD}:/backup alpine tar czf /backup/guacamole-db.tgz /data
copy .env $env:USERPROFILE\Documents\thinkcentre.env
docker compose unpause
```

Restore the archive into a new `postgres_data` volume only on first boot (init SQL runs only against an empty database). Keep `.env` with the backup.

## Security checklist

- Change `guacadmin` on first login
- Strong Windows (and TightVNC) passwords
- Prefer Cloudflare Access in front of Guacamole
- Never forward 3389/5900; never set `GUACAMOLE_BIND=0.0.0.0` on the ThinkCentre
- The Cloudflare hostname is enough for anyone on the internet to *reach* the login page — treat the URL as public

## Local development (Linux)

```bash
cp .env.example .env
# set POSTGRES_PASSWORD
docker compose up -d
```

Guacamole listens on `127.0.0.1:8080` by default. There is no Windows desktop in this environment; you should still get the login page. Seeded connections will fail to connect until they point at a real RDP/VNC host.

## License

Scripts and Compose files in this repository are provided as-is for running your own endpoint. Guacamole schema files under `guacamole/init/001-*.sql` and `002-*.sql` are Apache Guacamole (Apache License 2.0); see `NOTICE`.
