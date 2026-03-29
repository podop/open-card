# PayShield 10K Emulator — Administration Guide

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Installation](#installation)
4. [Initial Configuration (Production Flow)](#initial-configuration-production-flow)
5. [LMK Key Management](#lmk-key-management)
6. [Host Interface Configuration](#host-interface-configuration)
7. [Network Configuration](#network-configuration)
8. [HSM Identity](#hsm-identity)
9. [Authorization](#authorization)
10. [Command Management](#command-management)
11. [Configuration Console](#configuration-console)
12. [Configuration File Reference](#configuration-file-reference)
13. [Docker Deployment](#docker-deployment)
14. [Persistence and Backup](#persistence-and-backup)
15. [Security Considerations](#security-considerations)
16. [Comparison with Production PayShield 10K](#comparison-with-production-payshield-10k)
17. [Troubleshooting](#troubleshooting)

---

## Overview

The HSM Emulator is a software implementation of the Thales PayShield 10K Hardware Security Module host command interface. It implements the same TCP wire protocol, command set, and configuration model as the production PayShield 10K, enabling development, integration testing, and certification without physical HSM hardware.

The emulator consists of two binaries:

| Binary | Purpose |
|--------|---------|
| `hsm-emulator` | TCP server — listens for host commands on the configured port |
| `hsm-console` | Interactive CLI — mirrors the production PayShield serial/SSH configuration console |

**What it emulates:**
- Full PayShield TCP wire protocol (2-byte or 4-byte length-prefixed framing)
- 20 LMK key pairs with variant-based key encryption
- 20+ host commands (key generation, PIN, CVV, EMV, MAC, random)
- Production configuration flow (LMK loading, authorization, host interface setup)
- Key Check Values (KCV) computed identically to production

**What it does not emulate:**
- Physical tamper protection and tamper response
- Smart card reader hardware (simulated via hex input in console)
- FIPS 140-2/3 certified random number generation (uses Go `crypto/rand`)
- Hardware-accelerated cryptographic operations
- Clustered/redundant HSM configurations
- Full TR-31 key block format (variant scheme only)

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    hsm-emulator                           │
│                                                          │
│  ┌──────────┐    ┌───────────┐    ┌──────────────────┐   │
│  │  TCP     │───▶│ Processor │───▶│ LMK Store        │   │
│  │  Server  │    │ (router)  │    │ (20 key pairs)   │   │
│  │  :1500   │    │           │    │                  │   │
│  └──────────┘    │ ┌───────┐ │    │ Encrypt/Decrypt  │   │
│                  │ │Diag.  │ │    │ under LMK        │   │
│                  │ │KeyMgmt│ │    │                  │   │
│                  │ │PIN    │ │    │ KCV computation   │   │
│                  │ │Crypto │ │    └──────────────────┘   │
│                  │ │EMV    │ │                           │
│  ┌──────────┐    │ │Random │ │    ┌──────────────────┐   │
│  │  Config  │◀──▶│ └───────┘ │    │ config.json      │   │
│  │  Manager │    └───────────┘    │ lmk.bin          │   │
│  └──────────┘                     └──────────────────┘   │
└──────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────┐
│                    hsm-console                            │
│                                                          │
│  Interactive CLI that reads/writes config.json + lmk.bin │
│  Mirrors production PayShield serial/SSH console menus   │
└──────────────────────────────────────────────────────────┘
```

---

## Installation

### Build from Source

```bash
cd hsm-emulator

# Build both binaries
go build -o hsm-emulator ./cmd/hsm-emulator
go build -o hsm-console  ./cmd/hsm-console
```

### Docker

```bash
docker build -t hsm-emulator .

# Run with test LMK and auto-authorize (development)
docker run -d -p 1500:1500 -v hsmdata:/data hsm-emulator

# Run without auto-authorize (requires console setup)
docker run -d -p 1500:1500 -v hsmdata:/data hsm-emulator \
  --test-lmk --config /data/config.json --lmk /data/lmk.bin
```

### Docker Compose

The emulator is included in the project's `docker-compose.yml` as the `hsm` service:

```yaml
hsm:
  build:
    context: ./hsm-emulator
  ports:
    - "1500:1500"
  volumes:
    - hsmdata:/data
```

Start with:
```bash
docker-compose up -d hsm
```

---

## Initial Configuration (Production Flow)

The emulator mirrors the production PayShield 10K configuration flow. On a production device, an administrator connects to the console via serial cable or SSH (port 2222) and performs the following steps. The emulator replicates this flow through the `hsm-console` CLI and the `hsm-emulator` command-line flags.

### Production PayShield 10K Setup Sequence

| Step | Production HSM | Emulator Equivalent |
|------|----------------|---------------------|
| 1. Physical installation | Rack mount, power on | `docker run` or `go run` |
| 2. Connect to console | Serial cable / SSH port 2222 | Run `hsm-console` CLI |
| 3. Login | Admin smart card + PIN | (no auth required on emulator) |
| 4. Configure network | Set IP, subnet, gateway, DNS | Console menu 3 (Network) or config.json |
| 5. Load LMK keys | 3 custodians insert smart cards | Console menu 1 → option 3 (components) |
| 6. Verify LMK | Print check values, compare | Console menu 1 → option 4 |
| 7. Configure host port | Set TCP port, header length, max connections | Console menu 2 or CLI flags |
| 8. Set HSM identity | Serial, firmware version | Console menu 4 |
| 9. Authorize | Enter authorization code via console | Console menu 1 → option 5, or `--auto-authorize` flag |
| 10. HSM online | Begins accepting host commands | Server starts listening on configured port |

### Quick Start (Development)

For development, skip the full flow and use flags:

```bash
# Fastest: test LMK + auto-authorize
./hsm-emulator --test-lmk --auto-authorize

# With persistence
./hsm-emulator --test-lmk --auto-authorize \
  --config config.json --lmk lmk.bin

# Custom port and header length
./hsm-emulator --test-lmk --auto-authorize \
  --port 1500 --header 4
```

### Full Setup (Mirrors Production)

```bash
# Step 1: Run the console to configure
./hsm-console --config config.json --lmk lmk.bin

# Step 2: In the console:
#   1 → Security → 1 (Generate New LMK)    or  3 (Load from Components)
#   2 → Host Interface → set port, header, connections
#   3 → Network → set bind address
#   1 → Security → 5 (Authorize HSM)
#   0 → Exit

# Step 3: Start the server with the saved config
./hsm-emulator --config config.json --lmk lmk.bin
```

---

## LMK Key Management

### What are LMK Keys?

Local Master Keys (LMK) are the root cryptographic keys stored inside the HSM. All other keys (ZMK, ZPK, TPK, PVK, CVK, etc.) are encrypted under LMK pairs before being stored externally. The PayShield 10K uses 20 LMK key pairs (numbered 00-01 through 38-39), where each key is a double-length DES key (16 bytes).

### LMK Pair Assignments

| LMK Pair | Key Types Encrypted | Purpose |
|----------|--------------------|----|
| 00-01 | (Master LMK) | Root of trust |
| 04-05 | ZMK, CSK | Zone Master Keys |
| 06-07 | ZPK | Zone PIN Keys |
| 14-15 | PVK, TMK, TPK, CVK | PIN/Card verification and terminal keys |
| 16-17 | TAK | Terminal Authentication Keys |
| 20-21 | (Reserved) | — |
| 22-23 | TEK | Terminal Encryption Keys |
| 24-25 | RSA keys | Asymmetric key storage |
| 26-27 | ZAK | Zone Authentication Keys |
| 28-29 | ZEK | Zone Encryption Keys |
| 30-31 | DEK | Data Encryption Keys |
| 34-35 | AES keys | AES key storage |
| 36-37 | AES wrapping keys | Key wrapping |
| 38-39 | Key block protection | TR-31 key blocks |

### Loading LMK — Three Methods

#### Method 1: Test LMK (Development Only)

```bash
./hsm-emulator --test-lmk --auto-authorize
```

Loads a deterministic LMK set derived from known seed values. Every emulator instance using `--test-lmk` produces identical keys. This means keys encrypted under LMK on one instance can be decrypted on another — useful for shared test environments.

**WARNING:** Test LMK keys are publicly derivable. Never use them for any data with real security requirements.

#### Method 2: Random LMK (Recommended for Isolated Testing)

Via console:
```
Security → 1. Generate New LMK Set
```

Or programmatically — the LMK store generates 20 pairs of cryptographically random keys using Go's `crypto/rand`. The generated keys are saved to `lmk.bin` for persistence.

#### Method 3: Component Loading (Mirrors Production Ceremony)

In production, LMK keys are loaded through a key ceremony involving 3 key custodians. Each custodian contributes a key component via a smart card. The final LMK is formed by XORing all 3 components together.

The emulator simulates this via the console:

```
Security → 3. Load LMK from Smart Card Components

Custodian 1 — Enter component hex (640 bytes = 1280 hex chars)
Or type 'random' to generate: random
Generated component 1: a3f7b2c1e9d80456...

Custodian 2 — Enter component hex (640 bytes = 1280 hex chars)
Or type 'random' to generate: random
Generated component 2: 5e1a3b7c82f9d045...

Custodian 3 — Enter component hex (640 bytes = 1280 hex chars)
Or type 'random' to generate: random
Generated component 3: c80e5f2a1b34d789...

LMK loaded from 3 components
```

Each component is 640 bytes (20 pairs × 32 bytes). Components can be:
- Entered as hex strings (from real smart card exports)
- Generated randomly by typing `random`
- Mixed (some real, some random)

### Verifying LMK — Check Values

After loading LMK, always verify the check values:

```
Security → 4. Print LMK Check Values

LMK Check Values:
─────────────────────────
LMK 00-01: 28369B
LMK 02-03: B2CC38
LMK 04-05: 87D090
...
```

A Key Check Value (KCV) is computed by encrypting 8 zero bytes with the key using 3DES-ECB and taking the first 3 bytes (6 hex chars). If you're using the same LMK on multiple emulator instances, the KCVs must match exactly.

### Exporting and Importing LMK

```
Security → 7. Export LMK to File
Save path [lmk.bin]: /backup/lmk-2026-03-28.bin
LMK exported to /backup/lmk-2026-03-28.bin
```

To load an exported LMK file:
```bash
./hsm-emulator --lmk /backup/lmk-2026-03-28.bin --auto-authorize
```

The LMK file is a raw binary file of exactly 640 bytes (20 pairs × 32 bytes).

---

## Host Interface Configuration

The host interface is the TCP socket that accepts PayShield host commands from client applications.

### Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| TCP Port | 1500 | Standard PayShield host command port |
| Header Length | 2 | Length-prefix size in bytes. PayShield 9000 uses 2, PayShield 10K supports 2 or 4 |
| Max Connections | 64 | Maximum concurrent TCP connections |
| Command Timeout | 30 | Seconds before idle connection read times out |

### Setting via CLI Flags

```bash
./hsm-emulator --port 1500 --header 2 --test-lmk --auto-authorize
```

### Setting via Console

```
Host Interface Settings
  1. TCP Port:         1500
  2. Header Length:    2 bytes
  3. Max Connections:  64
  4. Command Timeout:  30 seconds
```

### Setting via config.json

```json
{
  "hostPort": 1500,
  "headerLength": 2,
  "maxConnections": 64,
  "commandTimeout": 30
}
```

### Header Length — 2 vs 4 Bytes

The header is a binary length prefix sent before each message. It specifies the byte count of the message body that follows.

- **2 bytes** (big-endian `uint16`): Maximum message size 65,535 bytes. Used by PayShield 9000 and PayShield 10K in compatibility mode. This is the most common setting.
- **4 bytes** (big-endian `uint32`): Maximum message size ~4GB. Used by PayShield 10K native mode.

**Important:** The header length must match between the HSM and all connected clients. If the Secrets Manager is configured with `headerLength: 2`, the emulator must also use `headerLength: 2`.

---

## Network Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| Bind Address | `0.0.0.0` | Interface to listen on. `0.0.0.0` = all interfaces |
| Console Port | 2222 | Reserved for future SSH-based console (not yet active) |

In Docker, the bind address should remain `0.0.0.0`. Port mapping is handled by Docker's `-p` flag or docker-compose `ports` directive.

---

## HSM Identity

These fields identify the emulator to clients that query HSM status. They are returned in NC (Diagnostics) and NO (Status) responses.

| Parameter | Default | Description |
|-----------|---------|-------------|
| Model | `PayShield 10K (Emulator)` | Reported in NO response |
| Serial Number | `EMUL-0000-0001` | 14-character serial |
| Firmware Version | `0650-E000` | Reported in NC response after LMK KCV |
| LMK Identifier | `0` | LMK set index (0-9). Production HSMs can store multiple LMK sets |

To make the emulator appear as a specific production model:

```
HSM Identity
  1. Model:            PayShield 10K
  2. Serial Number:    A000-0000-00F1
  3. Firmware Version: 0650-E000
  4. LMK Identifier:   0
```

---

## Authorization

A PayShield HSM must be explicitly authorized before it will process host commands. This is a security measure — if the HSM is physically compromised, an attacker cannot use it without the authorization code.

### States

| State | Host Commands | Description |
|-------|---------------|-------------|
| NOT AUTHORIZED | Rejected (error `90`) | HSM will only respond to NC (diagnostics) |
| AUTHORIZED | Accepted | HSM processes all enabled commands |

### Authorizing

**Via CLI flag** (development):
```bash
./hsm-emulator --test-lmk --auto-authorize
```

**Via console** (mirrors production):
```
Security → 5. Authorize HSM
Enter authorization code (or 'confirm'): confirm
HSM AUTHORIZED — ready to process host commands
```

In production, the authorization code is derived from the LMK and requires a physical smart card. In the emulator, any string of 4+ characters or the word `confirm` is accepted.

### Deauthorizing

```
Security → 6. Deauthorize HSM
HSM DEAUTHORIZED — host commands will be rejected
```

After deauthorization, all host commands except NC will return error code `90` (HSM not authorized).

---

## Command Management

Individual commands can be disabled without deauthorizing the entire HSM. This is useful for testing error handling in client applications.

### Disabling a Command

```
Command Management → 1. Disable a command
Command code to disable (e.g. EI): EI
Command EI disabled
```

Disabled commands return error code `68` (Command disabled).

### Enabling a Command

```
Command Management → 2. Enable a command
Command code to enable: EI
Command EI enabled
```

### Audit Logging

When enabled (default: `true`), every command processed is logged with:
- Command code and response code
- Error code
- Processing duration
- Client IP address

```
[HSM] A0 → A1 err=00 (90.669µs) from 192.168.127.1:17285
[HSM] NC → ND err=00 (151.228µs) from 192.168.127.1:57295
[HSM] N0 → N1 err=00 (11.151µs) from 192.168.127.1:48759
```

Toggle via console:
```
Command Management → 4. Audit logging: true
```

---

## Configuration Console

The `hsm-console` is a standalone interactive CLI that creates and edits the `config.json` and `lmk.bin` files used by the `hsm-emulator` server.

### Running

```bash
./hsm-console --config config.json --lmk lmk.bin
```

### Menu Structure

```
Main Menu
├── 1. Security (LMK, Authorization)
│   ├── 1. Generate New LMK Set
│   ├── 2. Load Test LMK (deterministic)
│   ├── 3. Load LMK from Smart Card Components
│   ├── 4. Print LMK Check Values
│   ├── 5. Authorize HSM
│   ├── 6. Deauthorize HSM
│   └── 7. Export LMK to File
├── 2. Host Interface Settings
│   ├── 1. TCP Port
│   ├── 2. Header Length
│   ├── 3. Max Connections
│   └── 4. Command Timeout
├── 3. Network Settings
│   ├── 1. Bind Address
│   └── 2. Console Port
├── 4. HSM Identity
│   ├── 1. Model
│   ├── 2. Serial Number
│   ├── 3. Firmware Version
│   └── 4. LMK Identifier
├── 5. Display Status
├── 6. Command Management
│   ├── 1. Disable a command
│   ├── 2. Enable a command
│   ├── 3. List disabled commands
│   └── 4. Toggle audit logging
└── 0. Exit
```

### Using Console with Docker

To run the console against the Docker volume:

```bash
# Attach to the running container
docker exec -it acs-server-hsm-1 ./hsm-console \
  --config /data/config.json --lmk /data/lmk.bin
```

Note: Configuration changes made via the console do not take effect on the running `hsm-emulator` server until it is restarted (config is loaded at startup).

---

## Configuration File Reference

### config.json

```json
{
  "hostAddress": "0.0.0.0",
  "hostPort": 1500,
  "consolePort": 2222,
  "headerLength": 2,
  "maxConnections": 64,
  "commandTimeout": 30,
  "messageLengthLE": false,
  "firmwareVersion": "0650-E000",
  "serialNumber": "EMUL-0000-0001",
  "model": "PayShield 10K (Emulator)",
  "lmkIdentifier": 0,
  "authorized": true,
  "lmkLoaded": true,
  "auditEnabled": true,
  "disabledCommands": {}
}
```

### lmk.bin

Raw binary file, exactly 640 bytes. Contains 20 LMK key pairs, each 32 bytes (left key 16 bytes + right key 16 bytes), concatenated in order: pair 00-01, pair 02-03, ..., pair 38-39.

---

## Docker Deployment

### Default Configuration

The Docker image defaults to:
```
--test-lmk --auto-authorize --config /data/config.json --lmk /data/lmk.bin
```

This means the container starts fully operational with test LMK keys on first run.

### Environment Overrides

Override entrypoint arguments:

```bash
docker run -d -p 1500:1500 -v hsmdata:/data hsm-emulator \
  --lmk /data/lmk.bin --config /data/config.json --port 1500 --header 4
```

### Volume Persistence

Mount `/data` to persist configuration and LMK keys across container restarts:

```yaml
volumes:
  - hsmdata:/data
```

On first start with `--test-lmk`, the emulator generates `lmk.bin` and `config.json` in `/data`. On subsequent starts, it loads from these files.

### Health Check

The Docker health check uses a simple `true` command. For TCP-based health checking from another service, send an NC command (see USER.md for protocol details).

### Networking

From other Docker containers on the same network, connect to `hsm:1500`:

```go
conn, err := net.Dial("tcp", "hsm:1500")
```

From the host machine, connect to `localhost:1500`.

---

## Persistence and Backup

### What to Back Up

| File | Contents | Criticality |
|------|----------|-------------|
| `lmk.bin` | 640-byte LMK key material | **Critical** — all keys encrypted under LMK become unrecoverable if LMK is lost |
| `config.json` | HSM configuration | Low — can be recreated via console |

### Backup Procedure

```bash
# Docker
docker cp acs-server-hsm-1:/data/lmk.bin ./backup/
docker cp acs-server-hsm-1:/data/config.json ./backup/

# Native
cp lmk.bin config.json /secure-backup/
```

### Restoring

```bash
# Docker
docker cp ./backup/lmk.bin acs-server-hsm-1:/data/
docker cp ./backup/config.json acs-server-hsm-1:/data/
docker restart acs-server-hsm-1

# Native
./hsm-emulator --config /secure-backup/config.json --lmk /secure-backup/lmk.bin
```

---

## Security Considerations

### Emulator vs Production

| Aspect | Production PayShield | Emulator |
|--------|---------------------|----------|
| Key storage | Tamper-resistant hardware | Memory + plaintext file (`lmk.bin`) |
| Random source | Hardware TRNG (FIPS 140-2) | Go `crypto/rand` (OS CSPRNG) |
| Physical security | Tamper mesh, zeroize on intrusion | None |
| Smart card auth | Required for LMK loading | Simulated (hex input) |
| Authorization | Requires physical presence | CLI flag or console command |
| Audit trail | Signed, non-repudiable | stdout logging |
| Certification | PCI HSM, FIPS 140-2 Level 3 | None |

### Recommendations

1. **Never use the emulator for production cryptographic operations** — it provides no physical security guarantees.
2. **Never use `--test-lmk` outside of development** — the test LMK set is deterministic and publicly known.
3. **Protect `lmk.bin`** — anyone with this file can decrypt all keys encrypted under the LMK.
4. **Use random LMK** for integration testing environments that process realistic (but non-production) data.
5. **Do not expose port 1500** to untrusted networks — the host command interface has no authentication.

---

## Comparison with Production PayShield 10K

### Configuration Parameters

| Parameter | Production | Emulator | Match? |
|-----------|-----------|----------|--------|
| Host TCP port | 1500 (configurable) | 1500 (configurable) | Yes |
| Header length | 2 or 4 | 2 or 4 | Yes |
| Max connections | Configurable | Configurable | Yes |
| LMK pairs | 20 | 20 | Yes |
| LMK loading | Smart card ceremony (3 custodians) | Console with hex components or random | Simulated |
| Authorization | Smart card + PIN | Console command or CLI flag | Simulated |
| Firmware version | Hardware-dependent | Configurable string | Cosmetic |
| Command timeout | Configurable | Configurable | Yes |
| Command enable/disable | Per-command | Per-command | Yes |
| Audit logging | Built-in, signed | stdout logging | Simplified |

### Wire Protocol

| Aspect | Production | Emulator | Match? |
|--------|-----------|----------|--------|
| Framing | `[length][body]` | `[length][body]` | Exact |
| Length encoding | Big-endian | Big-endian | Exact |
| Command codes | 2-char ASCII | 2-char ASCII | Exact |
| Response format | `resp_code(2) + err_code(2) + data` | `resp_code(2) + err_code(2) + data` | Exact |
| Error codes | Standard PayShield codes | Same codes | Exact |
| Response code convention | Last char +1 (A0→A1) | Last char +1 | Exact |

---

## Troubleshooting

### HSM returns error 90 for all commands

The HSM is not authorized. Either:
- Start with `--auto-authorize` flag
- Use the console to authorize: Security → 5. Authorize HSM

### HSM returns error 13 for all commands

LMK keys are not loaded. Either:
- Start with `--test-lmk` flag
- Use the console to load LMK: Security → 1/2/3

### HSM returns error 68 for a specific command

The command has been disabled. Use the console to re-enable it:
Command Management → 2. Enable a command

### Connection refused on port 1500

- Verify the emulator is running: `docker logs acs-server-hsm-1`
- Check the configured port matches: `--port 1500`
- In Docker, ensure port mapping: `-p 1500:1500`

### Client receives wrong response length

The header length setting must match between client and emulator. If the client uses 4-byte headers but the emulator uses 2-byte headers, responses will be misinterpreted. Set with `--header 2` or `--header 4`.

### LMK check values don't match between instances

If you need matching LMK across instances, either:
- Use `--test-lmk` on all instances (deterministic, identical)
- Copy the same `lmk.bin` file to all instances

### Config changes not taking effect

The `hsm-emulator` server loads config at startup. After making changes via `hsm-console`, restart the server:
```bash
docker restart acs-server-hsm-1
```

### Connection drops after timeout

Idle connections are kept alive, but if no command is sent within the configured `commandTimeout` (default 30 seconds), the next read will timeout. The connection remains open — send another command to resume.
