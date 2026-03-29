# HSM Emulator — User Guide

## Table of Contents

1. [Overview](#overview)
2. [Wire Protocol Specification](#wire-protocol-specification)
3. [Connecting to the HSM](#connecting-to-the-hsm)
4. [Command / Response Format](#command--response-format)
5. [Error Codes](#error-codes)
6. [Supported Commands Reference](#supported-commands-reference)
   - [Diagnostics](#diagnostics)
   - [Key Management](#key-management)
   - [PIN Operations](#pin-operations)
   - [CVV Operations](#cvv-operations)
   - [Data Encryption / Decryption](#data-encryption--decryption)
   - [MAC Generation](#mac-generation)
   - [EMV Operations](#emv-operations)
   - [Random Data Generation](#random-data-generation)
7. [Key Scheme Prefixes](#key-scheme-prefixes)
8. [Key Types](#key-types)
9. [Code Examples](#code-examples)
10. [Integration with Secrets Manager](#integration-with-secrets-manager)
11. [Testing with netcat / openssl](#testing-with-netcat--openssl)
12. [Troubleshooting](#troubleshooting)

---

## Overview

This document describes how to connect to and use the Thales PayShield 10K HSM Emulator as an application developer. It covers the wire protocol, all supported host commands, request/response field layouts, and code examples in multiple languages.

The emulator implements the same TCP host-command interface as a production PayShield 10K. Any client library or application that speaks the PayShield protocol can connect to this emulator with zero code changes — just point it at the emulator's address and port.

### Capabilities

| Category | Commands |
|----------|----------|
| Diagnostics | NC (Health Check), NO (HSM Status) |
| Key Management | A0 (Generate Key), A6 (Import Key), A8 (Export Key), BU (Key Check Value), B0 (Translate Key Scheme), EI (Generate RSA Key Pair) |
| PIN Operations | BA (Encrypt Clear PIN), DA (Verify PIN — IBM), DC (Verify PIN — Visa PVV), DE (Generate PIN Offset), DG (Generate PVV), CA (Translate PIN ZPK→ZPK), FA (Translate PIN Block) |
| CVV Operations | CW (Generate CVV), CY (Verify CVV) |
| Data Encryption | M0 (Encrypt Data Block), M2 (Decrypt Data Block) |
| MAC | MS (Generate MAC) |
| EMV | KQ (Verify ARQC / Generate ARPC) |
| Random | N0 (Generate Random Data) |

---

## Wire Protocol Specification

The PayShield host-command protocol uses **TCP** with a **length-prefixed binary framing** scheme.

### Message Frame

```
┌─────────────────────┬──────────────────────────────────────┐
│  Length Prefix       │  Message Body                        │
│  (2 or 4 bytes)     │  (variable length)                   │
│  Big-endian uint     │  ASCII command code + fields         │
└─────────────────────┴──────────────────────────────────────┘
```

#### Length Prefix

- **2-byte mode** (default): The first 2 bytes are a big-endian unsigned 16-bit integer indicating the length of the message body in bytes.
- **4-byte mode**: The first 4 bytes are a big-endian unsigned 32-bit integer.

The length prefix does **not** include itself — it counts only the message body bytes that follow.

#### Message Body

The message body is an ASCII string containing:

```
<command_code><field_1><field_2>...<field_n>
```

- **Command code**: Always 2 ASCII characters (e.g., `NC`, `A0`, `CW`).
- **Fields**: Variable-length, concatenated with no separators (unless the field format specifies a delimiter like `;`).
- All hexadecimal values use **uppercase** characters (`A`–`F`, `0`–`9`).

### Response Frame

Responses use the same framing:

```
┌─────────────────────┬──────────────────────────────────────┐
│  Length Prefix       │  Response Body                       │
│  (2 or 4 bytes)     │                                      │
└─────────────────────┴──────────────────────────────────────┘
```

The response body is:

```
<response_code><error_code>[<response_fields>]
```

- **Response code**: 2 characters. Convention: last character of the request code incremented by 1 (e.g., `NC` → `ND`, `A0` → `A1`, `CW` → `CX`).
- **Error code**: 2-character decimal string (`"00"` = success).
- **Response fields**: Present only when error code is `"00"`.

### Byte Order

All multi-byte integers (length prefix) are **big-endian** (network byte order).

### Connection Semantics

- **Persistent connections**: The emulator keeps connections open for multiple commands. There is no need to reconnect between commands.
- **Idle timeout**: Connections idle for longer than the configured `commandTimeout` (default: 30 seconds) will be kept alive but polled. The connection is dropped only on read errors.
- **Max connections**: The emulator enforces a configurable limit (default: 256). Connections beyond this limit are rejected immediately.
- **No TLS**: The emulator speaks plain TCP. For encrypted transport, use a TLS proxy or SSH tunnel.

---

## Connecting to the HSM

### Default Endpoint

| Parameter | Default Value |
|-----------|---------------|
| Host | `0.0.0.0` (all interfaces) |
| Port | `1500` |
| Header Length | `2` bytes |

### Docker Compose

When using the provided `docker-compose.yml`, the HSM emulator is reachable at:

```
Host: localhost
Port: 1500
```

From other Docker services on the same network:

```
Host: hsm
Port: 1500
```

### Connection Checklist

1. Verify the emulator is running and healthy (`docker ps` or check logs).
2. Confirm LMK keys are loaded (send `NC` command — if error code is `00`, the HSM is operational).
3. Confirm the HSM is authorized (send `NO` — first byte of response data is `1` if ready).
4. Begin sending commands.

---

## Command / Response Format

### Request Structure

```
[2/4-byte length prefix][command_code (2 chars)][command_data (variable)]
```

### Response Structure

```
[2/4-byte length prefix][response_code (2 chars)][error_code (2 chars)][response_data (variable)]
```

### Example: NC (Diagnostics)

**Request** (hex bytes on the wire, 2-byte header):

```
00 02 4E 43
│    │  └──── "NC" (ASCII 0x4E 0x43)
│    └─────── body length = 2
└──────────── length prefix (big-endian)
```

**Response** (hex bytes on the wire):

```
00 11 4E 44 30 30 41 42 43 44 45 46 30 30 30 30 30 30 30 30 30
│    │  └──┘ └──┘ └───────────────────────────────────────────┘
│    │  "ND"  "00"  LMK_check(6) + firmware(9)
│    └─────── body length = 17
└──────────── length prefix
```

Decoded response body: `ND00ABCDEF000000000` where:
- `ND` = response code
- `00` = success
- `ABCDEF` = LMK pair 00-01 check value (6 hex chars)
- `000000000` = firmware version (9 chars)

---

## Error Codes

| Code | Meaning | Description |
|------|---------|-------------|
| `00` | No error | Command completed successfully |
| `01` | Verification failure | PIN, CVV, or ARQC verification failed |
| `02` | Bad key type | Invalid or unsupported key type code |
| `03` | Bad key length | Key length does not match scheme |
| `04` | Bad source key | Source key decryption failed |
| `05` | Bad message length | Request too short or fields missing |
| `06` | Bad key block | Malformed key block |
| `10` | Parity error | DES key has incorrect parity |
| `11` | Key not found | Referenced key does not exist |
| `12` | Bad components | Key component validation failed |
| `13` | No LMK | LMK keys not loaded |
| `15` | Bad input | Invalid input parameter |
| `20` | PIN block error | Cannot parse or decrypt PIN block |
| `21` | Bad PIN pad | Invalid PIN padding |
| `26` | Bad decimalisation table | Decimalisation table format invalid |
| `27` | Bad scheme | Unknown key scheme prefix |
| `41` | Internal error | Unexpected processing error |
| `68` | Command disabled | Command has been disabled by admin |
| `90` | Not authorized | HSM has not been authorized |

---

## Supported Commands Reference

### Diagnostics

#### NC — Perform Diagnostics (NC → ND)

Health check command. Always responds even when the HSM is not authorized.

**Request:**

| Field | Length | Description |
|-------|--------|-------------|
| *(none)* | 0 | No additional fields required |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| LMK Check Value | 6 | Hex | KCV of LMK pair 00-01 |
| Firmware Version | 9 | ASCII | e.g., `0007-E000` |

**Example:**

```
Request:  NC
Response: ND00ABCDEF00007-E000
               ^^^^^^ ^^^^^^^^^
               KCV    Firmware
```

---

#### NO — HSM Status (NO → NP)

Returns detailed HSM status. Always responds even when not authorized.

**Request:**

| Field | Length | Description |
|-------|--------|-------------|
| *(none)* | 0 | No additional fields required |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Status | 1 | ASCII digit | `0` = not ready, `1` = ready |
| Serial Number | 14 | ASCII (left-aligned, space-padded) | HSM serial number |
| Model | 30 | ASCII (left-aligned, space-padded) | HSM model name |

**Example:**

```
Request:  NO
Response: NP001A1234-5678   PayShield 10K
               ^ ^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
               1 Serial(14)     Model(30)
```

---

### Key Management

#### A0 — Generate a Key (A0 → A1)

Generates a random DES/3DES key and returns it encrypted under the appropriate LMK pair.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Mode | 1 | ASCII digit | `0` = generate, `1` = generate + export |
| Key Type | 3 | Decimal | See [Key Types](#key-types) table |
| Key Scheme | 1 | ASCII | `Z`=single, `U`=double, `T`=triple, `X`=double variant |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Scheme | 1 | ASCII | Echo of requested scheme |
| Key (under LMK) | 16/32/48 | Hex | Encrypted key |
| Key Check Value | 6 | Hex | KCV (3DES-ECB encrypt of 8 zero bytes, first 3 bytes) |

**Example:**

```
Request:  A00002U
          │││   │
          ││└───┴── key_type=002 (ZPK), scheme=U (double-length)
          │└─────── mode=0 (generate)
          └──────── command code

Response: A100U1234567890ABCDEF1234567890ABCDEFAABB00
              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^
              key under LMK (32 hex)           KCV
```

---

#### A6 — Import Key under ZMK (A6 → A7)

Imports a key encrypted under a Zone Master Key (ZMK) and re-encrypts it under LMK.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Type | 3 | Decimal | Target key type |
| ZMK (under LMK) | 1+16/32/48 | Scheme prefix + Hex | Zone Master Key |
| Key (under ZMK) | 16/32 | Hex | Key to import |
| Key Scheme | 1 | ASCII | Target scheme for storage |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key (under LMK) | 1+32 | `U` + Hex | Re-encrypted key with scheme prefix |
| Key Check Value | 6 | Hex | KCV of the imported key |

---

#### A8 — Export Key under ZMK (A8 → A9)

Exports a key (stored under LMK) re-encrypted under a ZMK for transport to another HSM.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Type | 3 | Decimal | Key type of the key being exported |
| ZMK (under LMK) | 1+16/32/48 | Scheme prefix + Hex | Zone Master Key |
| Key (under LMK) | 1+16/32/48 | Scheme prefix + Hex | Key to export |
| Scheme | 1 | ASCII | Export scheme |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key (under ZMK) | 1+32 | `U` + Hex | Key encrypted under ZMK |
| Key Check Value | 6 | Hex | KCV of the key |

---

#### BU — Generate Key Check Value (BU → BV)

Computes the Key Check Value (KCV) for a key stored under LMK. The KCV is the first 3 bytes of 3DES-ECB encryption of 8 zero bytes using the clear key.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Type | 3 | Decimal | Key type |
| Key (under LMK) | 1+16/32/48 | Scheme prefix + Hex | Encrypted key |
| Delimiter | 1 | `;` | Separator |
| Check Length | 1 | `1` | KCV length flag |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Check Value | 6 | Hex | KCV (3 bytes) |

**Example:**

```
Request:  BU002U1234567890ABCDEF1234567890ABCDEF;1
Response: BV00AABB00
```

---

#### B0 — Translate Key Scheme (B0 → B1)

Re-encrypts a key from one LMK variant/scheme to another.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Type | 3 | Decimal | Key type |
| Source Scheme | 1 | ASCII | Current scheme prefix |
| Key (under LMK) | 16/32/48 | Hex | Key (without scheme prefix) |
| Dest Scheme | 1 | ASCII | Target scheme prefix |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key (under LMK) | 1+16/32/48 | Scheme + Hex | Key in new scheme |

---

#### EI — Generate RSA Key Pair (EI → EJ)

Generates an RSA key pair. The public key is returned in DER format; the private key is encrypted under LMK pair 24-25.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Key Length | 4 | Decimal | Bit length (512–4096, e.g., `2048`) |
| Key Type | 2 | Decimal | RSA key usage type |
| Public Exponent | 2 | Decimal | Exponent selector |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Public Key Length | 4 | Decimal | Length of public key hex string |
| Public Key | variable | Hex | DER-encoded PKIX public key |
| Private Key Length | 4 | Decimal | Length of encrypted private key hex |
| Private Key (under LMK) | variable | Hex | PKCS#8 private key encrypted under LMK |

---

### PIN Operations

#### BA — Encrypt Clear PIN (BA → BB)

Takes a clear-text PIN and account number, builds an ISO 9564-1 Format 0 PIN block, and encrypts it under LMK.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| PIN Length | 2 | Decimal | Length of PIN (04–12) |
| PIN | 4–12 | Decimal digits | Clear-text PIN |
| Account Number | 12 | Decimal digits | Rightmost 12 digits of PAN (excl. check digit) |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| PIN (under LMK) | 32 | Hex | Encrypted PIN block |

**Example:**

```
Request:  BA041234123456789012
          ││ │    │
          ││ │    └── account_number (12 digits)
          ││ └─────── PIN "1234"
          │└──────── PIN length = 04
          └───────── command code

Response: BB00AABBCCDD11223344AABBCCDD11223344
```

---

#### DA — Verify Terminal PIN, IBM Method (DA → DB)

Verifies a PIN encrypted under a Terminal PIN Key (TPK) using the IBM offset method.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| TPK (under LMK) | 1+16/32 | Scheme + Hex | Terminal PIN Key |
| PIN Block | 16 | Hex | PIN block encrypted under TPK |
| PIN Block Format | 2 | Decimal | `01` = ISO Format 0 |
| Check Length | 2 | Decimal | PIN verification length |
| PAN | 12 | Decimal | Account number (rightmost 12) |
| Decimalisation Table | 16 | Hex digits | IBM decimalisation table |
| Validation Data | variable | ASCII | Validation data for natural PIN |
| Offset | variable | Decimal | PIN offset |

**Response (on success):**

| Field | Length | Description |
|-------|--------|-------------|
| *(empty)* | 0 | Error code `00` = PIN verified |

Error code `01` means verification failed.

---

#### DC — Verify Terminal PIN, Visa PVV Method (DC → DD)

Verifies a PIN encrypted under TPK using the Visa PVV method.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| TPK (under LMK) | 1+16/32 | Scheme + Hex | Terminal PIN Key |
| PVK (under LMK) | 1+16/32 | Scheme + Hex | PIN Verification Key |
| PIN Block | 16 | Hex | PIN block encrypted under TPK |
| PIN Block Format | 2 | Decimal | `01` = ISO Format 0 |
| PVKI | 1 | Decimal | PVV Key Indicator |
| PVV | 4 | Decimal | Expected PIN Verification Value |
| PAN | 12 | Decimal | Account number |

**Response (on success):**

| Field | Length | Description |
|-------|--------|-------------|
| *(empty)* | 0 | Error code `00` = PIN verified |

---

#### DE — Generate PIN Offset, IBM Method (DE → DF)

Computes the IBM PIN offset (difference between customer PIN and natural PIN).

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| PVK (under LMK) | 1+16/32 | Scheme + Hex | PIN Verification Key |
| PIN Block | 16 | Hex | Encrypted PIN block |
| PIN Block Format | 2 | Decimal | PIN block format code |
| Check Length | 2 | Decimal | Offset length (typically `04`) |
| PAN | 12 | Decimal | Account number |
| Decimalisation Table | 16 | Hex | IBM decimalisation table |
| Validation Data | variable | ASCII | Validation data |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Offset | 4–12 | Decimal | PIN offset value |

---

#### DG — Generate PVV (DG → DH)

Generates a 4-digit Visa PIN Verification Value.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| TPK (under LMK) | 1+16/32 | Scheme + Hex | Terminal PIN Key |
| PIN Block | 16 | Hex | PIN block encrypted under TPK |
| PIN Block Format | 2 | Decimal | Format code |
| PVK (under LMK) | 1+16/32 | Scheme + Hex | PIN Verification Key |
| PAN | 12 | Decimal | Account number |
| PVKI | 1 | Decimal | PVV Key Indicator |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| PVV | 4 | Decimal | Computed PIN Verification Value |

---

#### CA — Translate PIN Block ZPK to ZPK (CA → CB)

Re-encrypts a PIN block from one Zone PIN Key to another. Used during PIN routing between acquirer and issuer.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Source ZPK (under LMK) | 1+16/32 | Scheme + Hex | Source encryption key |
| Dest ZPK (under LMK) | 1+16/32 | Scheme + Hex | Destination encryption key |
| Max PIN Length | 2 | Decimal | Maximum PIN length (`12`) |
| PIN Block | 16 | Hex | PIN block under source ZPK |
| Source Format | 2 | Decimal | Source PIN block format |
| Dest Format | 2 | Decimal | Destination PIN block format |
| PAN | 12 | Decimal | Account number |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| PIN Block Length | 2 | Decimal | Always `16` |
| PIN Block | 16 | Hex | PIN block under destination ZPK |

---

#### FA — Translate PIN Block (FA → FB)

Translates a PIN block between encryption keys with optional format change. Uses the same parameter layout as CA.

---

### CVV Operations

#### CW — Generate CVV (CW → CX)

Generates a 3-digit Card Verification Value using the Visa/Mastercard CVV algorithm.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| CVK (under LMK) | 1+32 | Scheme + Hex | Card Verification Key (CVK_A ∥ CVK_B) |
| PAN | variable | Decimal | Primary Account Number |
| Delimiter | 1 | `;` | Separates PAN from remaining fields |
| Expiry Date | 4 | `YYMM` | Card expiry |
| Service Code | 3 | Decimal | Card service code |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| CVV | 3 | Decimal | Computed CVV |

**Example:**

```
Request:  CWU1234567890ABCDEF1234567890ABCDEF4111111111111111;2512101
                                                ^^^^^^^^^^^^^^^^ ^^^^ ^^^
                                                PAN              YYMM SvcCode

Response: CX00123
              ^^^
              CVV
```

**Algorithm overview:**

1. Build data block: PAN + expiry + service code, zero-padded to 32 hex chars.
2. Split into two 8-byte blocks (B1, B2).
3. DES-encrypt B1 with CVK_A (left 8 bytes of CVK).
4. XOR result with B2.
5. 3DES-encrypt with full CVK (CVK_A ∥ CVK_B).
6. Decimalize: scan hex output left-to-right, extract first 3 decimal digits (0–9). If fewer than 3 found, second pass maps A→0, B→1, ..., F→5.

---

#### CY — Verify CVV (CY → CZ)

Verifies a CVV by recomputing and comparing.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| CVK (under LMK) | 1+32 | Scheme + Hex | Card Verification Key |
| CVV | 3 | Decimal | CVV to verify |
| PAN | variable | Decimal | Primary Account Number |
| Delimiter | 1 | `;` | Separator |
| Expiry Date | 4 | `YYMM` | Card expiry |
| Service Code | 3 | Decimal | Card service code |

**Response:**

| Error Code | Meaning |
|------------|---------|
| `00` | CVV matches |
| `01` | CVV does not match |

---

### Data Encryption / Decryption

#### M0 — Encrypt Data Block (M0 → M1)

Encrypts a data block using 3DES with a key stored under LMK.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Mode | 2 | Decimal | `00` = ECB, `01` = CBC (zero IV) |
| Input Format | 2 | Decimal | Input data format |
| Output Format | 2 | Decimal | Output data format |
| Key Type | 3 | Decimal | Key type code |
| Key (under LMK) | 1+16/32/48 | Scheme + Hex | Encryption key |
| Data Length | 4 | Hex | Length of plaintext in bytes |
| Data | variable | Hex | Plaintext to encrypt |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Encrypted Data | variable | Hex | Ciphertext |

**Example:**

```
Request:  M0000000002U1234567890ABCDEF1234567890ABCDEF0008AABBCCDD11223344
          │  │  │  │  │                                │   │
          │  │  │  │  └── key (32 hex, double-length)  │   └── 8 bytes plaintext
          │  │  │  └───── key_type=002                 └────── data_len=0x0008
          │  │  └──────── output_format=00
          │  └─────────── input_format=00
          └────────────── mode=00 (ECB)

Response: M100FFEEDDCCBBAA9988
```

---

#### M2 — Decrypt Data Block (M2 → M3)

Decrypts a data block using 3DES. Same field layout as M0.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Mode | 2 | Decimal | `00` = ECB |
| Input Format | 2 | Decimal | Input data format |
| Output Format | 2 | Decimal | Output data format |
| Key Type | 3 | Decimal | Key type code |
| Key (under LMK) | 1+16/32/48 | Scheme + Hex | Decryption key |
| Data Length | 4 | Hex | Length of ciphertext in bytes |
| Data | variable | Hex | Ciphertext to decrypt |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Decrypted Data | variable | Hex | Plaintext |

---

### MAC Generation

#### MS — Generate MAC (MS → MT)

Generates a Message Authentication Code using ISO 9797-1 Algorithm 3 (Retail MAC / DES CBC-MAC with 3DES final round).

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Mode | 1 | ASCII | Generation mode |
| Key Type | 3 | Decimal | MAC key type |
| Key Length Flag | 1 | ASCII | Key length indicator |
| MAC Algorithm | 2 | Decimal | Algorithm selector |
| MAC Padding | 2 | Decimal | Padding method |
| Key (under LMK) | 1+16/32/48 | Scheme + Hex | MAC key |
| Data Length | 4 | Hex | Message length in bytes |
| Data | variable | Hex | Message to authenticate |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| MAC | 16 | Hex | 8-byte MAC value |

**Algorithm:**

1. Pad message to 8-byte boundary with zero bytes.
2. CBC-MAC: XOR each 8-byte block with previous output, then single-DES encrypt with left key half.
3. Final block: DES-decrypt with right key half, then DES-encrypt with left key half (Retail MAC construction).

---

### EMV Operations

#### KQ — Verify ARQC / Generate ARPC (KQ → KR)

Verifies an Application Request Cryptogram (ARQC) from an EMV chip card and generates an Application Response Cryptogram (ARPC).

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Scheme ID | 1 | Decimal | Cryptogram scheme |
| MK-AC (under LMK) | 1+16/32 | Scheme + Hex | Master Key for Application Cryptograms |
| *(remaining fields)* | variable | | PAN, PAN seq, ATC, UN, transaction data, ARQC |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| ARPC | 16 | Hex | Application Response Cryptogram (8 bytes) |

> **Note:** In the emulator, ARQC verification is simplified — it validates message structure and key decryption, then returns a random ARPC. Production HSMs perform full ARQC computation.

---

### Random Data Generation

#### N0 — Generate Random Data (N0 → N1)

Generates cryptographically secure random bytes.

**Request:**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Length | 4 | Decimal | Number of random bytes to generate (1–4096) |

**Response (on success):**

| Field | Length | Format | Description |
|-------|--------|--------|-------------|
| Random Data | variable | Hex | Random bytes (hex-encoded) |

**Example:**

```
Request:  N00016
          └── generate 16 random bytes

Response: N100A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1D2E3F4A5B6C7D8
              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
              16 bytes = 32 hex characters
```

---

## Key Scheme Prefixes

When keys appear in requests or responses, they are prefixed with a **scheme tag** that indicates the key length:

| Prefix | Meaning | Key Length | Hex Chars |
|--------|---------|------------|-----------|
| `Z` | Single-length DES | 8 bytes | 16 |
| `U` | Double-length 3DES | 16 bytes | 32 |
| `T` | Triple-length 3DES | 24 bytes | 48 |
| `X` | Double-length variant (key block) | 16 bytes | 32 |

**Examples:**

```
Z1234567890ABCDEF                → single-length (8 bytes = 16 hex)
U1234567890ABCDEF1234567890ABCDEF → double-length (16 bytes = 32 hex)
T1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF → triple-length (24 bytes = 48 hex)
```

---

## Key Types

Each key stored under LMK is associated with a **key type** that determines which LMK pair encrypts it. Common types:

| Key Type Code | Key Name | LMK Pair | Description |
|---------------|----------|----------|-------------|
| `000` | ZMK | 04-05 | Zone Master Key |
| `001` | ZPK | 06-07 | Zone PIN Key |
| `002` | TPK / PVK / TMK | 14-15 | Terminal PIN Key / PIN Verification Key |
| `003` | TAK | 16-17 | Terminal Auth Key |
| `004` | ZAK | 26-27 | Zone Auth Key |
| `005` | BDK | 28-29 | Base Derivation Key |
| `006` | ZCMK | 30-31 | Zone Control Master Key |
| `007` | ZEK / DEK | 30-31 | Zone Encryption Key |
| `008` | RSA-SK | 24-25 | RSA Private Key |
| `009` | HMAC | 24-25 | HMAC Key |
| `00A` | CVK | 14-15 | Card Verification Key |
| `00B` | KTM | 24-25 | Key Transport Master |

---

## Code Examples

### Go — Send NC Command

```go
package main

import (
    "encoding/binary"
    "fmt"
    "net"
)

func main() {
    conn, err := net.Dial("tcp", "localhost:1500")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Build NC command frame (2-byte header)
    body := []byte("NC")
    frame := make([]byte, 2+len(body))
    binary.BigEndian.PutUint16(frame[:2], uint16(len(body)))
    copy(frame[2:], body)

    // Send
    conn.Write(frame)

    // Read response header
    header := make([]byte, 2)
    conn.Read(header)
    respLen := binary.BigEndian.Uint16(header)

    // Read response body
    resp := make([]byte, respLen)
    conn.Read(resp)

    fmt.Printf("Response: %s\n", string(resp))
    // Output: ND00<kcv><firmware>
}
```

### Go — Generate Key (A0)

```go
func generateKey(conn net.Conn, keyType string, scheme string) (string, error) {
    // mode=0, keyType=002, scheme=U
    body := []byte("A00" + keyType + scheme)
    frame := make([]byte, 2+len(body))
    binary.BigEndian.PutUint16(frame[:2], uint16(len(body)))
    copy(frame[2:], body)

    conn.Write(frame)

    header := make([]byte, 2)
    conn.Read(header)
    respLen := binary.BigEndian.Uint16(header)

    resp := make([]byte, respLen)
    conn.Read(resp)

    // resp = "A1" + errorCode(2) + scheme(1) + key(32) + kcv(6)
    if string(resp[2:4]) != "00" {
        return "", fmt.Errorf("HSM error: %s", string(resp[2:4]))
    }

    keyData := string(resp[4:]) // scheme + encrypted key + KCV
    return keyData, nil
}
```

### Python — Send Commands

```python
import socket
import struct

def hsm_command(host, port, command):
    """Send a PayShield command and return the response body."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect((host, port))

    # Frame: 2-byte big-endian length prefix + body
    body = command.encode('ascii')
    frame = struct.pack('>H', len(body)) + body
    sock.sendall(frame)

    # Read response
    header = sock.recv(2)
    resp_len = struct.unpack('>H', header)[0]
    resp = sock.recv(resp_len).decode('ascii')

    sock.close()
    return resp

# Health check
resp = hsm_command('localhost', 1500, 'NC')
print(f"Response code: {resp[:2]}")   # "ND"
print(f"Error code:    {resp[2:4]}")  # "00"
print(f"LMK KCV:       {resp[4:10]}") # e.g., "ABCDEF"
print(f"Firmware:       {resp[10:]}")  # e.g., "0007-E000"

# Generate a double-length ZPK
resp = hsm_command('localhost', 1500, 'A00002U')
print(f"Key: {resp[5:37]}")   # 32 hex chars
print(f"KCV: {resp[37:43]}")  # 6 hex chars

# Generate 32 random bytes
resp = hsm_command('localhost', 1500, 'N00032')
print(f"Random: {resp[4:]}")  # 64 hex chars

# Generate CVV
cvk = 'U' + resp[5:37]  # use a real CVK here
pan = '4111111111111111'
cmd = f'CW{cvk}{pan};2512101'
resp = hsm_command('localhost', 1500, cmd)
print(f"CVV: {resp[4:]}")    # 3-digit CVV
```

### Java — Using Socket

```java
import java.io.*;
import java.net.*;
import java.nio.ByteBuffer;

public class HsmClient {

    private final String host;
    private final int port;

    public HsmClient(String host, int port) {
        this.host = host;
        this.port = port;
    }

    public String sendCommand(String command) throws IOException {
        try (Socket socket = new Socket(host, port)) {
            DataOutputStream out = new DataOutputStream(socket.getOutputStream());
            DataInputStream in = new DataInputStream(socket.getInputStream());

            // Write length-prefixed frame
            byte[] body = command.getBytes("ASCII");
            out.writeShort(body.length);  // 2-byte big-endian
            out.write(body);
            out.flush();

            // Read response
            int respLen = in.readUnsignedShort();
            byte[] resp = new byte[respLen];
            in.readFully(resp);
            return new String(resp, "ASCII");
        }
    }

    public static void main(String[] args) throws Exception {
        HsmClient client = new HsmClient("localhost", 1500);

        // Health check
        String resp = client.sendCommand("NC");
        System.out.println("Health: " + resp);

        // Generate ZPK (double-length)
        resp = client.sendCommand("A00002U");
        System.out.println("Key response: " + resp);

        // Generate 16 random bytes
        resp = client.sendCommand("N00016");
        System.out.println("Random: " + resp);
    }
}
```

### C# (.NET) — Using TcpClient

```csharp
using System;
using System.Net.Sockets;
using System.Text;

class HsmClient
{
    private readonly string _host;
    private readonly int _port;

    public HsmClient(string host, int port)
    {
        _host = host;
        _port = port;
    }

    public string SendCommand(string command)
    {
        using var client = new TcpClient(_host, _port);
        using var stream = client.GetStream();

        // Build frame: 2-byte big-endian length + body
        byte[] body = Encoding.ASCII.GetBytes(command);
        byte[] header = BitConverter.GetBytes((ushort)body.Length);
        if (BitConverter.IsLittleEndian)
            Array.Reverse(header);

        stream.Write(header, 0, 2);
        stream.Write(body, 0, body.Length);

        // Read response header
        byte[] respHeader = new byte[2];
        stream.Read(respHeader, 0, 2);
        if (BitConverter.IsLittleEndian)
            Array.Reverse(respHeader);
        int respLen = BitConverter.ToUInt16(respHeader, 0);

        // Read response body
        byte[] resp = new byte[respLen];
        int read = 0;
        while (read < respLen)
            read += stream.Read(resp, read, respLen - read);

        return Encoding.ASCII.GetString(resp);
    }
}
```

### Node.js — Using net Module

```javascript
const net = require('net');

function hsmCommand(host, port, command) {
    return new Promise((resolve, reject) => {
        const client = new net.Socket();
        client.connect(port, host, () => {
            const body = Buffer.from(command, 'ascii');
            const header = Buffer.alloc(2);
            header.writeUInt16BE(body.length, 0);
            client.write(Buffer.concat([header, body]));
        });

        let data = Buffer.alloc(0);
        client.on('data', (chunk) => {
            data = Buffer.concat([data, chunk]);
            if (data.length >= 2) {
                const respLen = data.readUInt16BE(0);
                if (data.length >= 2 + respLen) {
                    const resp = data.slice(2, 2 + respLen).toString('ascii');
                    client.destroy();
                    resolve(resp);
                }
            }
        });

        client.on('error', reject);
    });
}

// Usage
(async () => {
    const resp = await hsmCommand('localhost', 1500, 'NC');
    console.log('Response:', resp);

    const keyResp = await hsmCommand('localhost', 1500, 'A00002U');
    console.log('Key:', keyResp);
})();
```

---

## Integration with Secrets Manager

The HSM emulator integrates with the Secrets Manager service, which acts as a gateway. The Secrets Manager can:

1. **Proxy commands** to the HSM emulator via its HSM gateway API.
2. **Manage HSM endpoints** — add/remove/monitor emulator instances.
3. **Generate random data** for OTP and cryptographic operations via the HSM.
4. **Store keys** generated by the HSM under its own key management layer.

### Secrets Manager HSM API Routes

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/secrets/hsm/v1/admin/endpoints` | List HSM endpoints |
| `POST` | `/secrets/hsm/v1/admin/endpoints` | Add HSM endpoint |
| `POST` | `/secrets/hsm/v1/admin/endpoints/:id/test` | Test connectivity |
| `POST` | `/secrets/hsm/v1/admin/endpoints/:id/diagnostics` | Send NC command |
| `POST` | `/secrets/hsm/v1/admin/endpoints/:id/generate-key` | Generate key via A0 |
| `POST` | `/secrets/hsm/v1/admin/endpoints/:id/encrypt` | Encrypt data via M0 |
| `POST` | `/secrets/hsm/v1/admin/endpoints/:id/decrypt` | Decrypt data via M2 |
| `POST` | `/secrets/hsm/v1/admin/generate-random` | Generate random bytes via N0 |

### Connecting Secrets Manager to HSM

In `docker-compose.yml`, the HSM is available at `hsm:1500`. When creating an HSM endpoint in the Secrets Manager UI:

- **Address**: `hsm`
- **Port**: `1500`
- **Header Length**: `2`
- **Model**: `PayShield 10K`

---

## Testing with netcat / openssl

### Quick Health Check with Python One-Liner

```bash
python3 -c "
import socket, struct
s=socket.socket(); s.connect(('localhost',1500))
s.send(struct.pack('>H',2)+b'NC')
h=s.recv(2); l=struct.unpack('>H',h)[0]; print(s.recv(l))
s.close()
"
```

### Using a TCP Debugging Tool

Since the protocol is binary-framed (length prefix), raw `netcat` is not ideal. Use a tool that supports binary framing, or write a minimal script (see code examples above).

### Quick Smoke Test Sequence

1. **NC** — Verify HSM is alive. Expect `ND00...`.
2. **NO** — Check authorization status. Expect `NP001...` (status=1 means ready).
3. **A0** — Generate a ZPK. Expect `A100U<32 hex><6 hex>`.
4. **N0** — Generate random bytes. Expect `N100<hex>`.
5. **BU** — Compute KCV of the generated key. Expect `BV00<6 hex>` matching the KCV from step 3.
6. **CW** — Generate CVV with a test CVK.
7. **BA** — Encrypt a test PIN.

---

## Troubleshooting

### Connection Refused

- Verify the emulator is running: `docker ps | grep hsm`
- Check the port mapping: default is `1500`
- From another Docker container, use hostname `hsm` instead of `localhost`

### Error Code 90 (Not Authorized)

The HSM has not been authorized. Either:
- Start with `--auto-authorize` flag
- Use the HSM console to authorize (see ADMIN.md)
- In Docker, the default entrypoint includes `--auto-authorize`

### Error Code 13 (No LMK)

LMK keys have not been loaded. Either:
- Start with `--test-lmk` flag for deterministic test keys
- Start with `--lmk /path/to/lmk.bin` for a saved key file
- Use the console to generate or load LMK (see ADMIN.md)

### Error Code 68 (Command Disabled)

The command has been disabled by an administrator. Check the configuration file or console to re-enable it.

### Error Code 05 (Bad Message Length)

The request body is too short. Double-check:
- All required fields are present
- Field lengths match the specification
- Hex values use uppercase characters
- Key scheme prefix (`U`, `Z`, `T`, `X`) is included when required

### Error Code 10 (Parity Error)

A DES key failed odd parity validation. DES keys must have odd parity on each byte. The emulator enforces this when generating keys but may reject imported keys with bad parity.

### Responses Are Garbled

- Verify you're using the correct header length (2 or 4 bytes)
- Ensure length prefix is **big-endian**
- Read exactly the number of bytes indicated by the length prefix

### Connection Drops After One Command

This should not happen — the emulator supports persistent connections. Check:
- Your client is reading the full response before sending the next command
- No read timeout is set too aggressively on the client side
