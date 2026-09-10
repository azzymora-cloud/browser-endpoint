# Set this up from scratch

Follow this on the Windows PC that will stay at home and stay powered on. A friend can do the same on their own machine. You need about an hour, a domain, and a Cloudflare account (free plan is enough).

When you are done, you open `https://desktop.yourdomain.com` from any browser. No VPN app. No inbound router ports.

## 0. Buy / point a domain

1. Buy any cheap domain (or use one you already own).
2. Add the site to [Cloudflare](https://dash.cloudflare.com/) on the **Free** plan.
3. At the registrar, set nameservers to **exactly** the two Cloudflare nameservers shown for that zone.
4. Wait until Cloudflare shows the zone as **Active**. Do not continue the tunnel until it is Active.

You will pick two hostnames, for example:

| Hostname | What it is |
|---|---|
| `desktop.yourdomain.com` | Guacamole (the remote desktop) |
| `manage.yourdomain.com` | Optional agent console (reboot / Wake-on-LAN / connection log) |

## 1. Install Git and Docker Desktop

1. Install [Git for Windows](https://git-scm.com/download/win).
2. Install [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/) with the **WSL2** backend.
3. In Docker Desktop → Settings → General, enable **Start Docker Desktop when you log in**.
4. Sign in to Windows so Docker is actually running (whale icon in the tray).

## 2. Clone this repository

In PowerShell:

```powershell
git clone https://github.com/azzymora-cloud/browser-endpoint.git
cd browser-endpoint
```

Stay in this folder for the rest of the steps. Do not copy random files out of a website UI — clone the whole repo so `releases/thinkcentre-agent.exe` and `guacamole/init/` are on disk.

## 3. Prepare Windows (Administrator)

Open **PowerShell as Administrator** in the repo:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\setup-windows.ps1
```

That script:

- Enables **RDP** on Pro / Enterprise / Education, or installs **TightVNC** on Home
- Tightens the firewall so 3389 / 5900 are only reachable from loopback and private LAN/Docker ranges
- Turns off sleep / hibernate on AC and disables UPnP
- Turns on NIC Wake-on-LAN flags

On Home it will ask for a TightVNC password (8 characters max).

Confirm BIOS: **Wake on LAN**, and **After Power Loss = Power On** if you want the box to come back after an outage.

## 4. Create `.env`

```powershell
copy .env.example .env
notepad .env
```

Set a long random `POSTGRES_PASSWORD`. Leave:

```text
GUACAMOLE_BIND=127.0.0.1
GUACAMOLE_PORT=8080
CLOUDFLARE_TUNNEL_TOKEN=
```

Never commit `.env`.

## 5. Start Guacamole locally

Docker Desktop must be running.

```powershell
docker compose up -d
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080).

| Field | Value |
|---|---|
| Username | `guacadmin` |
| Password | `guacadmin` |

Then:

1. **Settings → Preferences** — change the `guacadmin` password. Do not use the Users admin “change password” page; it errors.
2. Click **ThinkCentre (RDP)** on Pro, or **ThinkCentre (VNC)** on Home.
3. Sign in with the **Windows** password (RDP) or the TightVNC password (Home) — not `guacadmin`.

Optional: Settings → Connections → ThinkCentre (RDP) → set **username** to your Windows account name so you only type the password.

If this local session works, the rest is only Cloudflare.

## 6. Named Cloudflare Tunnel

Do **not** install the Windows `cloudflared` service. Docker runs the connector.

Read [CLOUDFLARE.md](CLOUDFLARE.md) for the dashboard clicks. Short version:

1. [Zero Trust](https://one.dash.cloudflare.com/) → **Networks** → **Tunnels** → Create a tunnel → **Cloudflared**.
2. Name it something like `thinkcentre`.
3. Copy the **token** (the long `eyJ…` string after `cloudflared.exe service install`). You will **not** run that Windows command.
4. Add a **published application route**:
   - Subdomain e.g. `desktop`
   - Domain = pick your zone from the dropdown (do not only type it)
   - **Full hostname must be** `desktop.yourdomain.com`
   - Type **HTTP**, URL `localhost:8080`
5. Optional second route: `manage` → Type HTTP, URL `host.docker.internal:18765`
6. On the PC:

```powershell
.\scripts\setup-tunnel.ps1
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

Paste the token when asked. The connector should show **Healthy** in Zero Trust within a minute.

7. Confirm DNS: Cloudflare → your domain → **DNS** → a **proxied CNAME** `desktop` → `<tunnel-id>.cfargotunnel.com`. Saving the published route usually creates this. If `nslookup desktop.yourdomain.com` is NXDOMAIN, add that CNAME yourself (Proxied / orange cloud).

## 7. Cloudflare Access (email PIN)

The tunnel URL is otherwise public. Add a second gate: [CLOUDFLARE.md](CLOUDFLARE.md#cloudflare-access-email-pin).

Zero Trust → **Access** → **Applications** → Self-hosted → hostname `desktop.yourdomain.com` → policy: your email + **One-time PIN**.

Free plan: email PIN. SMS to a phone number is not available on Free.

## 8. Management agent

Still in an Administrator PowerShell in the repo:

```powershell
.\scripts\install-service.ps1
```

That installs the **ThinkCentreEndpoint** Windows service and a logon task that starts Docker Compose.

- Console: [http://127.0.0.1:18765](http://127.0.0.1:18765)
- Token: `%ProgramData%\ThinkCentreEndpoint\config.json`
- Connection log: `%ProgramData%\ThinkCentreEndpoint\logs\connections.jsonl`

Use **Start stack** in the console if Guacamole is down after a reboot. If you already have a tunnel token, start the overlay from the repo:

```powershell
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
```

## 9. Daily use

From another network (or your phone, not on this PC’s Wi‑Fi if you want a clean test):

1. Open `https://desktop.yourdomain.com`
2. Complete the Access email PIN if you enabled it
3. Log in to Guacamole
4. Click **ThinkCentre (RDP)** or **ThinkCentre (VNC)**
5. Enter the Windows / VNC password

The PC must be **on**, Docker Desktop **running**, and the `cloudflared` container **up**.

## 10. Do not do these

- Do not port-forward 3389, 5900, or 8080 on the router
- Do not set `GUACAMOLE_BIND=0.0.0.0`
- Do not run `cloudflared.exe service install …` on Windows (Docker already runs the connector)
- Do not use `trycloudflare.com` as the daily URL
- Do not skip changing `guacadmin`
