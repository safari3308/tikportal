# TikPortal — Project Memory & Agent Instructions

## Overview
**TikPortal** is an open-source, self-hosted, single-binary WireGuard VPN self-service portal for **MikroTik RouterOS v7**.
It manages one or more WireGuard networks mapped to existing RouterOS WireGuard interfaces, where routing/firewall policies remain managed on RouterOS and peer provisioning is managed by TikPortal.

## Tech Stack & Architecture
- **Backend**: Go (`go 1.24+`, binary at `/opt/homebrew/bin/go` on local macOS dev machine)
- **Database**: Pure-Go SQLite (`modernc.org/sqlite`), WAL mode enabled, single writer connection (`SetMaxOpenConns(1)`)
- **Frontend**: React 19 + TypeScript + Vite + TailwindCSS v4 (`web/`), built with Bun (`/opt/homebrew/bin/bun`)
- **Single-Binary Embedding**: [embed.go](embed.go) embeds `web/dist` into the Go binary (`./tikportal`)
- **RouterOS Integration**: RouterOS v7 HTTPS REST API (`/rest/interface/wireguard` and `/rest/interface/wireguard/peers`) in `internal/routeros/client.go`.
  - **Scope**: TikPortal **only adds, updates, or removes peers** on existing RouterOS WireGuard interfaces; it never modifies RouterOS routing tables, WAN interfaces, or firewall rules.
  - **Interface Filtering**: Controlled by `ROUTEROS_INTERFACE_TAG` (default `tikportal`, matched against RouterOS interface `comment`) and optional `ROUTEROS_ALLOWED_INTERFACES`.
  - **Mock Mode**: When `ROUTEROS_MOCK=true` (or `ROUTEROS_URL` is empty), `MockClient` simulates a RouterOS device with two WireGuard interfaces (`wg-main` and `wg-guest`).
- **Authentication**: OIDC (Zitadel, Keycloak, etc.) with PKCE in `internal/auth/auth.go`, plus `DEV_AUTH_ENABLED=true` local dev login fallback.

## Core Domain Rules
1. **Global User Quota**:
   - Peer limits apply **globally per user across all networks** (`default_max_peers_per_user` in `settings`, with optional per-user `max_peers_override` in `users`).
2. **IP Allocation & Reuse (IPAM)**:
   - `internal/ipam/ipam.go` allocates the lowest available host IP in the network's `subnet_cidr` (skipping network, `server_ip` gateway, and broadcast).
   - Deleting a peer removes it from RouterOS and `wg_peers`, immediately freeing its IP for reuse by subsequent connections.
3. **Key Provisioning & Zero Private-Key Storage**:
   - **Mode 1 (`server`)**: Backend generates Curve25519 keypair + PSK in memory, stores only `public_key` and `preshared_key`, and returns the `PrivateKey` once in the `.conf` and QR code response. Private keys are never persisted or cached on the server.
   - **Mode 2 (`custom`)**: User supplies their own `public_key`. Backend validates it is a 32-byte X25519 key and returns a `.conf` template (`PrivateKey = <INSERT_YOUR_PRIVATE_KEY>`) with **no QR code**.
   - **In-Place Key Regeneration (`POST /api/peers/{id}/regenerate`)**: Rotates a peer's keys on RouterOS (`PATCH /rest/interface/wireguard/peers/<id>`) and SQLite while preserving its `allocated_ip`.

## Build & Test Commands
```bash
# 1. Build frontend into web/dist
/opt/homebrew/bin/bun install --cwd web
/opt/homebrew/bin/bun run --cwd web build

# 2. Run Go unit tests
/opt/homebrew/bin/go test ./...

# 3. Build single-binary executable (with embedded frontend)
/opt/homebrew/bin/go build -o tikportal ./cmd/tikportal
```
