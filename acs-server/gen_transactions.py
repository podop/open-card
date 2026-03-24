#!/usr/bin/env python3
"""
Test transaction generator.

Visa   (2.3.1):
  Batch 1 — 3 Payment Authentication (PA)     → expected ARes transStatus=Y
  Batch 2 — 4 Non-Payment Authentication (NPA) → expected ARes transStatus=Y
  Batch 3 — 3 Challenge (PA, no browserIP)     → expected ARes transStatus=C, completed via OTP

Mastercard (2.3.1):
  Batch 4 — 6 3DS Requestor Initiated (3RI)   → expected ARes transStatus=Y
"""

import json
import re
import subprocess
import time
import urllib.error
import urllib.request
import uuid

ACS = "http://localhost:8443"
DOCKER_CONTAINER = "acs-server-acs-1"

GREEN  = "\033[92m"
RED    = "\033[91m"
YELLOW = "\033[33m"
CYAN   = "\033[96m"
MAGENTA = "\033[95m"
RESET  = "\033[0m"

# ── helpers ──────────────────────────────────────────────────────────────────

def post(path, body, parse_json=True):
    data = json.dumps(body).encode()
    req  = urllib.request.Request(
        ACS + path, data=data,
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            raw = r.read()
            return r.status, json.loads(raw) if parse_json else raw
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"error": raw.decode(errors="replace")}

def browser_areq(pan, version, merchant, amount, msg_category="01", include_browser_ip=True):
    body = {
        "messageType":                  "AReq",
        "messageVersion":               version,
        "messageCategory":              msg_category,
        "deviceChannel":                "02",
        "threeDSServerTransID":         str(uuid.uuid4()),
        "dsTransID":                    str(uuid.uuid4()),
        "acctNumber":                   pan,
        "merchantName":                 merchant,
        "merchantCountry":              "840",
        "merchantID":                   "MERCH001",
        "threeDSMethodNotificationURL": "https://example.com/notify",
        "browserAcceptHeader":          "text/html",
        "browserUserAgent":             "Mozilla/5.0 (test)",
        "browserLanguage":              "en-US",
        "browserColorDepth":            "24",
        "browserScreenHeight":          "900",
        "browserScreenWidth":           "1440",
        "browserTZ":                    -120,
        "browserJavaEnabled":           False,
        "browserJavascriptEnabled":     True,
    }
    if msg_category == "01":
        body["purchaseAmount"]   = amount
        body["purchaseCurrency"] = "840"
        body["purchaseExponent"] = 2
    if include_browser_ip:
        body["browserIP"] = "192.168.1.1"
    return body

def sdk_areq(pan, version, merchant, amount):
    """Build an app-channel (deviceChannel=01) AReq."""
    sdk_trans_id = str(uuid.uuid4())
    return {
        "messageType":          "AReq",
        "messageVersion":       version,
        "messageCategory":      "01",
        "deviceChannel":        "01",
        "threeDSServerTransID": str(uuid.uuid4()),
        "dsTransID":            str(uuid.uuid4()),
        "acctNumber":           pan,
        "merchantName":         merchant,
        "merchantCountry":      "840",
        "merchantID":           "MERCH001",
        "purchaseAmount":       amount,
        "purchaseCurrency":     "840",
        "purchaseExponent":     2,
        # SDK fields (sdkEncData decryptor is nil in dev — value ignored)
        "sdkAppID":             str(uuid.uuid4()),
        "sdkTransID":           sdk_trans_id,
        "sdkEncData":           "eyJhbGciOiJSU0EtT0FFUCIsImVuYyI6IkExMjhDQkMtSFMyNTYifQ.dummy",
        "sdkEphemPubKey":       '{"kty":"EC","crv":"P-256","x":"stub","y":"stub"}',
        "sdkMaxTimeout":        5,
        "sdkReferenceNumber":   "SDK-TEST-REF-001",
    }, sdk_trans_id

def complete_challenge_sdk(acs_trans_id, sdk_trans_id, msg_version="2.3.1"):
    """Send app-channel CReq then submit OTP. Returns final transStatus."""
    creq_body = {
        "messageType":          "CReq",
        "messageVersion":       msg_version,
        "threeDSServerTransID": str(uuid.uuid4()),
        "acsTransID":           acs_trans_id,
        "sdkTransID":           sdk_trans_id,
        "sdkCounterStoA":       "000001",
    }
    # CReq response is HTML (challenge page rendered for SDK WebView)
    status, _ = post("/v1/challenge/creq", creq_body, parse_json=False)
    if status not in (200, 201):
        print(f"    {RED}CReq failed HTTP {status}{RESET}")
        return None

    otp = otp_from_logs(acs_trans_id)
    if not otp:
        print(f"    {RED}OTP not found in logs for {acs_trans_id}{RESET}")
        return None
    print(f"    OTP: {CYAN}{otp}{RESET}")

    status2, resp2 = post("/v1/challenge/submit", {
        "acsTransID": acs_trans_id,
        "otpCode":    otp,
    })
    if status2 not in (200, 201):
        print(f"    {RED}OTP submit failed {status2}: {resp2}{RESET}")
        return None
    return resp2.get("transStatus")

def three_ri_areq(pan, version, merchant, amount, three_ri_ind):
    """Build a 3RI AReq (deviceChannel=03). No browser or SDK fields."""
    return {
        "messageType":          "AReq",
        "messageVersion":       version,
        "messageCategory":      "01",
        "deviceChannel":        "03",
        "threeDSServerTransID": str(uuid.uuid4()),
        "dsTransID":            str(uuid.uuid4()),
        "acctNumber":           pan,
        "merchantName":         merchant,
        "merchantCountry":      "840",
        "merchantID":           "MERCH001",
        "purchaseAmount":       amount,
        "purchaseCurrency":     "840",
        "purchaseExponent":     2,
        "threeRIInd":           three_ri_ind,
    }

def otp_from_logs(acs_trans_id, since_seconds=15):
    """Extract OTP for the given acsTransID from docker logs."""
    result = subprocess.run(
        ["docker", "logs", DOCKER_CONTAINER, "--since", f"{since_seconds}s"],
        capture_output=True, text=True,
    )
    combined = result.stdout + result.stderr
    pattern = re.compile(
        r'otp generated.*?"otp":\s*"(\d{6})".*?"acs_trans_id":\s*"' + re.escape(acs_trans_id) + r'"'
    )
    m = pattern.search(combined)
    if m:
        return m.group(1)
    pattern2 = re.compile(
        r'otp generated.*?"acs_trans_id":\s*"' + re.escape(acs_trans_id) + r'".*?"otp":\s*"(\d{6})"'
    )
    m2 = pattern2.search(combined)
    return m2.group(1) if m2 else None

def complete_challenge(acs_trans_id, msg_version="2.3.1"):
    """Send CReq then submit OTP. Returns final transStatus."""
    creq_body = {
        "messageType":          "CReq",
        "messageVersion":       msg_version,
        "threeDSServerTransID": str(uuid.uuid4()),
        "acsTransID":           acs_trans_id,
        "challengeWindowSize":  "05",
    }
    status, _ = post("/v1/challenge/creq", creq_body, parse_json=False)
    if status not in (200, 201):
        print(f"    {RED}CReq failed HTTP {status}{RESET}")
        return None

    otp = otp_from_logs(acs_trans_id)
    if not otp:
        print(f"    {RED}OTP not found in logs for {acs_trans_id}{RESET}")
        return None
    print(f"    OTP: {CYAN}{otp}{RESET}")

    status2, resp2 = post("/v1/challenge/submit", {
        "acsTransID": acs_trans_id,
        "otpCode":    otp,
    })
    if status2 not in (200, 201):
        print(f"    {RED}OTP submit failed {status2}: {resp2}{RESET}")
        return None
    return resp2.get("transStatus")


# ── Batch 1: Visa PA frictionless ────────────────────────────────────────────

print(f"\n{CYAN}{'='*60}{RESET}")
print(f"{CYAN}Batch 1 — Visa Payment Auth (2.3.1)  x3  [expect Y]{RESET}")
print(f"{CYAN}{'='*60}{RESET}")

visa_pa_pans = [
    ("4000000000000010", "Amazon Store"),
    ("4000000000000028", "Apple Online"),
    ("4000000000000036", "Netflix Service"),
]

for i, (pan, merchant) in enumerate(visa_pa_pans, 1):
    body = browser_areq(pan, "2.3.1", merchant, "005000", msg_category="01", include_browser_ip=True)
    status, resp = post("/v1/auth", body)
    ts = resp.get("transStatus", "?")
    ok = ts == "Y"
    mark = f"{GREEN}PASS{RESET}" if ok else f"{RED}FAIL{RESET}"
    print(f"  [{i}] {merchant:<22} PAN={pan}  transStatus={ts}  [{mark}]")
    if not ok:
        print(f"       HTTP {status}  {resp}")
    time.sleep(0.3)

# ── Batch 2: Visa NPA frictionless ───────────────────────────────────────────

print(f"\n{CYAN}{'='*60}{RESET}")
print(f"{CYAN}Batch 2 — Visa Non-Payment Auth (2.3.1)  x4  [expect Y]{RESET}")
print(f"{CYAN}{'='*60}{RESET}")

visa_npa = [
    ("4000000000000044", "Verify Identity",    "01"),  # add card
    ("4000000000000051", "Card on File Check", "02"),  # account verification
    ("4000000000000069", "Wallet Enroll",      "03"),  # add card
    ("4000000000000077", "Device Binding",     "04"),  # maintain card
]

for i, (pan, merchant, _) in enumerate(visa_npa, 1):
    # NPA: messageCategory=02, no purchaseAmount required
    body = browser_areq(pan, "2.3.1", merchant, "001000", msg_category="02", include_browser_ip=True)
    status, resp = post("/v1/auth", body)
    ts = resp.get("transStatus", "?")
    ok = ts == "Y"
    mark = f"{GREEN}PASS{RESET}" if ok else f"{RED}FAIL{RESET}"
    print(f"  [{i}] {merchant:<22} PAN={pan}  transStatus={ts}  [{mark}]")
    if not ok:
        print(f"       HTTP {status}  {resp}")
    time.sleep(0.3)

# ── Batch 3: Visa challenge (PA, no browserIP) ────────────────────────────────

print(f"\n{CYAN}{'='*60}{RESET}")
print(f"{CYAN}Batch 3 — Visa Challenge (2.3.1)  x3  [expect C -> Y]{RESET}")
print(f"{CYAN}{'='*60}{RESET}")

visa_challenge = [
    ("4000000000000085", "Big Ticket Store",  "050000"),
    ("4000000000000093", "Luxury Goods",      "099999"),
    ("4000000000000101", "Travel Agency",     "075000"),
]

for i, (pan, merchant, amount) in enumerate(visa_challenge, 1):
    # No browserIP -> risk engine triggers challenge
    body = browser_areq(pan, "2.3.1", merchant, amount, msg_category="01", include_browser_ip=False)
    status, resp = post("/v1/auth", body)
    ts    = resp.get("transStatus", "?")
    acs_id = resp.get("acsTransID", "")

    if ts != "C":
        mark = f"{RED}FAIL (expected C, got {ts}){RESET}"
        print(f"  [{i}] {merchant:<22} PAN={pan}  transStatus={ts}  [{mark}]")
        print(f"       HTTP {status}  {resp}")
        time.sleep(0.3)
        continue

    print(f"  [{i}] {merchant:<22} PAN={pan}  ARes=C -> completing challenge ...")
    time.sleep(0.5)

    final = complete_challenge(acs_id, msg_version="2.3.1")
    ok = final == "Y"
    mark = f"{GREEN}PASS (Y){RESET}" if ok else f"{RED}FAIL (got {final}){RESET}"
    print(f"       Final transStatus={final}  [{mark}]")
    time.sleep(0.3)

# ── Batch 4: Mastercard 3RI ───────────────────────────────────────────────────

print(f"\n{MAGENTA}{'='*60}{RESET}")
print(f"{MAGENTA}Batch 4 — Mastercard 3RI (2.3.1)  x6  [expect Y]{RESET}")
print(f"{MAGENTA}{'='*60}{RESET}")

mc_3ri = [
    ("5200000000000018", "Subscription Svc",  "001000", "01"),  # recurring
    ("5200000000000026", "Instalment Plan",   "005000", "02"),  # instalment
    ("5200000000000034", "Add Card Flow",     "001000", "03"),  # add card
    ("5200000000000042", "Card Maintenance",  "000100", "04"),  # maintain card info
    ("5200000000000059", "Account Verify",    "000100", "05"),  # account verification
    ("5200000000000067", "Top-Up Wallet",     "002500", "07"),  # top-up
]

for i, (pan, merchant, amount, ind) in enumerate(mc_3ri, 1):
    body = three_ri_areq(pan, "2.3.1", merchant, amount, ind)
    status, resp = post("/v1/auth", body)
    ts = resp.get("transStatus", "?")
    ok = ts == "Y"
    mark = f"{GREEN}PASS{RESET}" if ok else f"{YELLOW}NOTE (got {ts}){RESET}"
    ind_label = {
        "01": "recurring", "02": "instalment", "03": "add-card",
        "04": "maintain",  "05": "acct-verify","07": "top-up",
    }.get(ind, ind)
    print(f"  [{i}] {merchant:<22} PAN={pan}  3RIind={ind}({ind_label})  transStatus={ts}  [{mark}]")
    if status >= 400:
        print(f"       HTTP {status}  {resp}")
    time.sleep(0.3)

# ── Batch 5: Visa SDK (app channel) ──────────────────────────────────────────

print(f"\n{CYAN}{'='*60}{RESET}")
print(f"{CYAN}Batch 5 — Visa SDK / App-channel (2.3.1)  x3{RESET}")
print(f"{CYAN}{'='*60}{RESET}")

visa_sdk = [
    ("4000000000000119", "Mobile Pay App",   "003000", "frictionless"),
    ("4000000000000127", "In-App Store",     "009000", "frictionless"),
    ("4000000000000135", "SDK High Value",   "080000", "challenge"),
]

for i, (pan, merchant, amount, expected) in enumerate(visa_sdk, 1):
    body, sdk_trans_id = sdk_areq(pan, "2.3.1", merchant, amount)
    status, resp = post("/v1/auth", body)
    ts    = resp.get("transStatus", "?")
    acs_id = resp.get("acsTransID", "")

    if expected == "frictionless":
        ok = ts == "Y"
        mark = f"{GREEN}PASS{RESET}" if ok else f"{RED}FAIL (expected Y, got {ts}){RESET}"
        print(f"  [{i}] {merchant:<22} PAN={pan}  transStatus={ts}  [{mark}]")
        if not ok:
            print(f"       HTTP {status}  {resp}")

    else:  # challenge
        if ts != "C":
            mark = f"{RED}FAIL (expected C, got {ts}){RESET}"
            print(f"  [{i}] {merchant:<22} PAN={pan}  transStatus={ts}  [{mark}]")
            print(f"       HTTP {status}  {resp}")
        else:
            print(f"  [{i}] {merchant:<22} PAN={pan}  ARes=C -> completing SDK challenge ...")
            time.sleep(0.5)
            final = complete_challenge_sdk(acs_id, sdk_trans_id, msg_version="2.3.1")
            ok = final == "Y"
            mark = f"{GREEN}PASS (Y){RESET}" if ok else f"{RED}FAIL (got {final}){RESET}"
            print(f"       Final transStatus={final}  [{mark}]")

    time.sleep(0.3)

print(f"\n{CYAN}Done.{RESET}\n")