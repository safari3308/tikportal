# TikPortal

Self-hosted, single-binary **WireGuard VPN Self-Service Portal** for **MikroTik RouterOS v7**, built with **Go**, **React (TypeScript + TailwindCSS)**, **SQLite**, and **OIDC** (Zitadel, Keycloak, Authentik, etc.).

---

## Features

- **Multi-Network WireGuard Management**:
  - Map one or more existing RouterOS WireGuard interfaces (e.g., `wg-main`, `wg-guest`, or separate routing profiles) into user-selectable VPN networks.
  - **Safe Interface Filtering**: Only exposes RouterOS WireGuard interfaces whose comment contains `ROUTEROS_INTERFACE_TAG` (default: `tikportal`) and/or matches `ROUTEROS_ALLOWED_INTERFACES`, keeping your personal or site-to-site WireGuard interfaces isolated.
- **Global Per-User Peer Quotas**:
  - Admin configures a global default limit across all networks (e.g., 5 total peers per user), with optional per-user overrides.
- **Automatic IP Allocation & Reuse (IPAM)**:
  - Allocates the lowest available host IP in the network's CIDR. When a peer is deleted, its IP is immediately released and reused for subsequent connections.
- **Flexible Key Provisioning & In-Place Key Regeneration**:
  - **Mode 1 (Server-Generated)**: Generates Curve25519 keys in memory and displays a scannable mobile **QR Code** + `.conf` download once (private keys are never stored on the server).
  - **Mode 2 (User-Provided Public Key)**: Validates the user's public key and provides a `.conf` file template (`PrivateKey = <INSERT_YOUR_PRIVATE_KEY>`) without a QR code.
  - **In-Place Key Regeneration**: Users can rotate/regenerate keys for an existing peer (`PATCH /rest/interface/wireguard/peers/<id>`) to get a fresh QR code without deleting the peer or losing their allocated IP.
- **Dynamic WAN / NAT Traversal Ready**:
  - Configures `PersistentKeepalive` (default `25s`) on both the generated client `.conf` and the RouterOS peer record, and defaults client DNS to the WireGuard interface's router gateway IP.

---

## Preparing Your MikroTik Router (RouterOS v7.24+)

TikPortal uses the native **RouterOS v7 HTTPS REST API** (`/rest/interface/wireguard/...`) and only adds, updates, or removes peers on existing WireGuard interfaces.

### 1. Create & Assign an HTTPS Certificate for `www-ssl`
In newer RouterOS v7 versions, a server certificate with `key-usage=tls-server` cannot self-sign directly (`failure: CA not found`). Create a local Root CA first, sign the server certificate with it, and assign it to `www-ssl`:

```routeros
/certificate
# 1. Create and self-sign a local Root CA
add name=local-ca common-name=RouterOS-Local-CA days-valid=3650 key-usage=key-cert-sign,crl-sign
sign local-ca

# 2. Create the www-ssl server certificate and sign it with local-ca
add name=routeros-rest-cert common-name=router.local days-valid=3650 key-usage=tls-server
sign routeros-rest-cert ca=local-ca

# 3. Enable www-ssl (default port 443, or custom port e.g. 4443) with the signed certificate
/ip service
set www-ssl certificate=routeros-rest-cert port=443 disabled=no
```

> **Important**: In WinBox (`IP -> Services -> www-ssl`), verify the **Certificate** column shows `routeros-rest-cert` (not `none`). If it is `none`, the TLS handshake will time out.

### 2. Create a Least-Privilege API User for TikPortal
RouterOS v7's REST API authenticates through the internal `api` subsystem and classifies WireGuard `preshared-key` fields under the `sensitive` policy. Therefore, the user group **must** include all five permissions: **`rest-api,api,read,write,sensitive`**:

```routeros
/user group
add name=tikportal-group policy=rest-api,api,read,write,sensitive,!local,!telnet,!ssh,!ftp,!reboot,!policy,!test,!winbox,!password,!web,!sniff,!romon

/user
add name=tikportal-api group=tikportal-group password="YOUR_STRONG_PASSWORD" comment="TikPortal WireGuard Manager"
```

### 3. Allow TikPortal Through RouterOS Firewall (`input`) and `www-ssl`
Both `/ip service` (`available-from`, if restricted) **and** `/ip firewall filter` (`chain=input`) must permit traffic from the TikPortal container/host IP to your router's `www-ssl` port:

```routeros
# 1. If you restrict www-ssl 'available-from', append your TikPortal host/container IP
/ip service
set www-ssl available-from="192.168.88.0/24"

# 2. Allow TCP to www-ssl in the firewall input chain BEFORE your "Drop All Other Input" rule
/ip firewall filter
add action=accept chain=input comment="Allow TikPortal to RouterOS REST API" \
    src-address=192.168.88.10 dst-address=192.168.88.1 protocol=tcp dst-port=443 \
    place-before=[find where comment="Drop All Other Input"]
```

> **Docker Networking Note**: If TikPortal runs in Docker's default bridge network (`ports: - "8080:8080"`), outbound connections to RouterOS will come from the **Docker host's IP**, not a container IP—unless you attach the container to a `macvlan`/`ipvlan` network with a dedicated IP.

### 4. Tag WireGuard Interfaces for TikPortal
By default (`ROUTEROS_INTERFACE_TAG=tikportal`), TikPortal only discovers and manages RouterOS WireGuard interfaces whose `comment` contains `tikportal` (case-insensitive):

```routeros
/interface wireguard
add name=wg-main  listen-port=51820 mtu=1420 comment="TikPortal - Main VPN"
add name=wg-guest listen-port=51821 mtu=1420 comment="TikPortal - Guest VPN"
```

---

## Deployment with Docker Compose

1. Copy [.env.example](.env.example) to `.env` and fill in your values:
   ```bash
   cp .env.example .env
   ```
   > **Gotcha with `$` in Passwords**: If `ROUTEROS_PASSWORD` or `SESSION_SECRET` contains a `$` character, wrap the value in single quotes in `.env` (e.g., `ROUTEROS_PASSWORD='my$secret'`) so Docker Compose does not interpolate it as an empty variable.
   >
   > **Disabling Dev Login**: Set `DEV_AUTH_ENABLED=false` in `.env` once your OIDC provider (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`) is configured so users can only sign in via OIDC.

2. Start the container:
   ```bash
   docker compose up -d
   ```

---

## Troubleshooting Common Errors

| Symptom / Error Message | Cause | Fix |
| :--- | :--- | :--- |
| `context deadline exceeded (Client.Timeout exceeded while awaiting headers)` | 1. RouterOS `/ip firewall filter` (`chain=input`) is dropping packets to `www-ssl`<br>2. `www-ssl` `available-from` doesn't include the container/host source IP<br>3. `www-ssl` has `certificate=none` | Allow the `www-ssl` TCP port in `chain=input` before your drop rule, include the source IP in `www-ssl available-from`, and set a valid certificate on `www-ssl`. |
| `RouterOS API error (500): {"detail":"std failure: not allowed (9)"}` | The RouterOS user group is missing the `sensitive` policy required to set WireGuard `preshared-key` values. | Add `sensitive` to `tikportal-group` policy (`policy=rest-api,api,read,write,sensitive,...`). |
| MikroTik Log: `login failure for user tikportal-api via api` | 1. The RouterOS user group has `!api` disabled (RouterOS REST API uses `api` internally)<br>2. Password in `.env` contains an unescaped `$` | 1. Add `api` to `tikportal-group` policy.<br>2. Check `docker exec tikportal printenv ROUTEROS_PASSWORD`. |
| Interface does not appear under **Discovered WireGuard Interfaces** | Interface comment on RouterOS does not contain `ROUTEROS_INTERFACE_TAG` (default: `tikportal`). | Add `TikPortal` to the interface's comment in WinBox, or set `ROUTEROS_INTERFACE_TAG=*` in `.env`. |

---

## Local Development

```bash
# 1. Build the React frontend into web/dist
cd web && bun install && bun run build && cd ..

# 2. Run Go unit tests
go test ./...

# 3. Run the Go backend (reads .env automatically)
go run ./cmd/tikportal
```
