# Browser remote endpoint

Turn an always-on **Windows PC** into a desktop you open from **any browser** — including hotel, school, and work Wi‑Fi that blocks VPNs, UPnP, and apps like Parsec.

The PC stays a normal Windows machine. [Apache Guacamole](https://guacamole.apache.org/) paints the desktop in HTML5. [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) carries that page over **outbound HTTPS on port 443**. You never forward RDP, never enable UPnP, and you do not install a VPN on the laptop or phone you are borrowing.

This repository is the full stack: Compose files, Windows setup scripts, a small management agent (reboot, Wake-on-LAN, connection audit log, session kill switch), and the Cloudflare pieces.

**From-scratch walkthrough for a friend:** [docs/SETUP.md](docs/SETUP.md)

## Why this works where Parsec and VPNs fail

Parsec, UPnP, and many VPN clients need **inbound ports**, **UDP hole punching**, or a protocol the firewall already flags.

This stack does the opposite:

1. The Windows PC only **dials out** to Cloudflare on 443 (same as opening a website).
2. Your browser on the restricted network does **not** run a VPN. It loads a normal `https://` hostname.
3. The desktop is relayed **inside that HTTPS session** (WebSockets over TLS), not a peer-to-peer UDP path.

Tailscale / WireGuard is **not** the way in. Networks that “block VPNs” often block those by name or UDP. Cloudflare looks like ordinary web traffic.

**Still will not work if** the client network blocks Cloudflare entirely, a proxy allows HTTPS but strips WebSockets, or you have not completed a captive portal. The home ISP only needs outbound 443.

```text
Browser on locked-down Wi-Fi
        |  HTTPS :443
        v
Cloudflare edge  (+ optional Access email PIN)
        |  existing outbound tunnel
        v
cloudflared in Docker on the PC
        v
Guacamole (localhost only)
        v
Windows desktop (RDP or VNC on the PC, not on the internet)
```

## What you need

- An always-on Windows PC (Home or Pro). A Lenovo ThinkCentre is what this was built on; any similar box works.
- [Docker Desktop for Windows](https://docs.docker.com/desktop/setup/install/windows-install/) with the WSL2 backend. Turn on **Start Docker Desktop when you log in**.
- A free [Cloudflare](https://dash.cloudflare.com/) account and a **domain** on Cloudflare (required for a stable URL). `trycloudflare.com` URLs change every restart.
- The PC left powered on, not sleeping.

## Quick start (this PC)

Admin PowerShell from a clone of this repo:

```powershell
git clone https://github.com/azzymora-cloud/browser-endpoint.git
cd browser-endpoint
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\setup-windows.ps1
copy .env.example .env
notepad .env   # set POSTGRES_PASSWORD; leave GUACAMOLE_BIND=127.0.0.1
docker compose up -d
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). First login is `guacadmin` / `guacadmin`. Change that password immediately under **Settings → Preferences** (not the Users admin page).

Then expose it with a named Cloudflare Tunnel (see [docs/CLOUDFLARE.md](docs/CLOUDFLARE.md)):

```powershell
.\scripts\setup-tunnel.ps1
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

Install the management agent (connection log, stack start/stop, reboot, Wake-on-LAN, session HUD kill switch):

```powershell
.\scripts\install-service.ps1
```

Open [http://127.0.0.1:18765](http://127.0.0.1:18765). The token is in `%ProgramData%\ThinkCentreEndpoint\config.json`.

## Windows edition

| Edition | Desktop protocol | Guacamole connection |
|---|---|---|
| Pro, Enterprise, Education | Remote Desktop (RDP, port 3389) | **ThinkCentre (RDP)** |
| Home | TightVNC (port 5900) — Windows Home cannot host RDP | **ThinkCentre (VNC)** |

`scripts/setup-windows.ps1` detects the edition. RDP and VNC stay off the public internet. The firewall allows those ports only from loopback and RFC1918 (so Docker Desktop can reach `host.docker.internal`). Do **not** port-forward 3389 or 5900 on the router.

The RDP password is the **Windows** password, not `guacadmin`. On Pro, signing in over RDP from the same console session can kick the local desktop — test from another device.

## Docs

| Doc | What it covers |
|---|---|
| [docs/SETUP.md](docs/SETUP.md) | Clone → Windows prep → Guacamole → tunnel → Access → daily use |
| [docs/CLOUDFLARE.md](docs/CLOUDFLARE.md) | Named tunnel, public hostnames, DNS, Cloudflare Access email PIN |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Docker stopped, DNS cache, RDP auth, connector inactive |

## Repository layout

| Path | Role |
|---|---|
| `docker-compose.yml` | `guacd`, Postgres, Guacamole, session HUD nginx (localhost only) |
| `docker-compose.tunnel.yml` | `cloudflared` overlay (needs `CLOUDFLARE_TUNNEL_TOKEN`) |
| `guacamole/init/` | Official Guacamole 1.6.0 schema plus desktop connection seeds |
| `guacamole/hud/` | Parsec-style overlay: Ctrl+Alt+Del, captive mouse, kill focused app |
| `guacamole/connection-template.md` | RDP/VNC settings if you add connections by hand |
| `scripts/setup-windows.ps1` | Home vs Pro, firewall, always-on power, UPnP off, WoL |
| `scripts/enable-wol.ps1` | NIC magic-packet wake + BIOS checklist |
| `scripts/setup-tunnel.ps1` | Save the named-tunnel token |
| `scripts/install-service.ps1` | Windows service + logon task for the stack |
| `scripts/build-release.sh` | Windows agent `.exe` + MSI |
| `agent/` | Management service (status, stack, reboot, WoL, connection log, HUD kill switch) |
| `releases/thinkcentre-agent.exe` | Prebuilt agent used by `install-service.ps1` |
| `cloudflare/config.yml.example` | Locally-managed tunnel (token method is preferred) |
| `.env.example` | Secrets template — copy to `.env`, never commit `.env` |

## Keep the box reachable

- Leave the PC powered when you can; the setup script turns off sleep and hibernate on AC
- Enable **Start Docker Desktop when you log in**
- Enable Wake on LAN in BIOS and in the agent console so a magic packet can bring it back
- Do not put the NIC to sleep in Device Manager → network adapter → Power Management
- UPnP is disabled on purpose. This design never uses it
- Reboot from `http://127.0.0.1:18765` (or your manage hostname) when the desktop is stuck

## Connection audit log

The agent service polls Guacamole history, Guacamole web logins, local RDP accepts, and cloudflared request lines. It writes JSON lines to:

```text
%ProgramData%\ThinkCentreEndpoint\logs\connections.jsonl
```

The same events appear in the agent console under **Incoming connections**. Nothing extra is opened on the network.

## Session HUD (while connected)

A small **P** control sits at the top center of the desktop view (Guacamole client page), similar to Parsec’s overlay:

- **Ctrl+Alt+Del** — sends that key combination into the remote session
- **Captive mouse** — pointer-locks the cursor so it cannot leave the browser window (Esc releases it)
- **Kill focused app** — the Windows agent force-stops the process that owns the foreground window, for when an app freezes and blocks input. Explorer and other shell processes are refused

The kill switch talks to the agent on loopback (`127.0.0.1:18765`) through nginx. Update the agent after pulling this change (`scripts/update-agent.ps1` as Administrator).

## Security

- Change `guacadmin` on first login (**Settings → Preferences**)
- Strong Windows (and TightVNC) passwords
- Put [Cloudflare Access](docs/CLOUDFLARE.md#cloudflare-access-email-pin) in front of Guacamole (email one-time PIN)
- Never forward 3389/5900; never set `GUACAMOLE_BIND=0.0.0.0`
- Treat the public hostname as public until Access is on
- Treat the agent token like a password. Put Access on the manage hostname too

## Backup

Guacamole users and connections live in the `postgres_data` Docker volume. Tunnel credentials live in `.env` (`CLOUDFLARE_TUNNEL_TOKEN`).

```powershell
docker compose pause
docker run --rm -v thinkcentre_postgres_data:/data -v ${PWD}:/backup alpine tar czf /backup/guacamole-db.tgz /data
copy .env $env:USERPROFILE\Documents\thinkcentre.env
docker compose unpause
```

Restore the archive into a new `postgres_data` volume only on first boot (init SQL runs only against an empty database). Keep `.env` with the backup.

## License

Scripts, Compose files, agent, and documentation in this repository are MIT. Guacamole schema files under `guacamole/init/001-*.sql` and `002-*.sql` are Apache Guacamole (Apache License 2.0); see `NOTICE`.
