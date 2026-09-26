#!/usr/bin/env python3
"""MVP-20 canonical workload (specs/quality/performance.md).

300 foreground clients at 10 s polls (150 queue trackers, 150 diners who also
read their bill and occasionally the menu), 20 staff at 3 s polls (host board,
kitchen board, cashier bill reads), about 10 mutations/s (assisted orders,
assisted queue joins, settlement begin/reopen with some confirmations), 50
members reading loyalty, and a manager reading reports/audit. Prints one JSON
line per route plus a summary. Standard library only.
"""
import json, random, statistics, sys, threading, time, urllib.error, urllib.request, uuid

base, ids_file, warm, dur = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
ids = json.load(open(ids_file))
BR = ids["branch"]
lock = threading.Lock()
samples, sizes, errors = {}, {}, {}
t0 = time.time()
end = t0 + warm + dur

def measuring():
    return warm <= time.time() - t0 < warm + dur

def call(route, method, path, headers, body=None):
    req = urllib.request.Request(base + path, method=method, headers=headers,
                                 data=None if body is None else json.dumps(body).encode())
    s = time.perf_counter()
    try:
        r = urllib.request.urlopen(req, timeout=15)
        b = r.read(); st = r.status
    except urllib.error.HTTPError as e:
        b = e.read(); st = e.code
    except Exception as e:  # network-level failure: counted as status 0
        b = b""; st = 0
        with lock:
            errors[type(e).__name__] = errors.get(type(e).__name__, 0) + 1
    ms = (time.perf_counter() - s) * 1000
    if measuring():
        with lock:
            samples.setdefault(route, []).append((ms, st))
            sizes.setdefault(route, []).append(len(b))
    try:
        return st, json.loads(b)
    except Exception:
        return st, None

staff = {"Cookie": "__Host-tf_staff=" + ids["staff"]}
staffw = {**staff, "Origin": "http://perf.test", "Content-Type": "application/json"}

def sleep(mean):
    time.sleep(mean * random.uniform(0.8, 1.2))

def tracker(tok, ticket):
    time.sleep(random.uniform(0, 10))
    h = {"Cookie": "__Host-tf_guest=" + tok}
    while time.time() < end:
        call("GET queue ticket (tracker)", "GET", f"/api/v1/queue-tickets/{ticket}", h)
        sleep(10)

def diner(tok, visit):
    time.sleep(random.uniform(0, 10))
    h = {"Cookie": "__Host-tf_guest=" + tok}
    i = 0
    while time.time() < end:
        call("GET visit orders (diner)", "GET", f"/api/v1/visits/{visit}/orders?limit=100", h)
        if i % 2 == 0:
            call("GET visit bill (diner)", "GET", f"/api/v1/visits/{visit}/bill", h)
        if i % 6 == 0:
            call("GET menu category (diner)", "GET", f"/api/v1/branches/{BR}/menu?category_id={random.choice(ids['categories'])}",
                 {"Accept-Encoding": "gzip"})
        i += 1
        sleep(10)

def host():
    time.sleep(random.uniform(0, 3))
    while time.time() < end:
        call("GET queue board (host)", "GET", f"/api/v1/branches/{BR}/queue-tickets?limit=100", staff)
        call("GET tables (host)", "GET", f"/api/v1/branches/{BR}/tables", staff)
        sleep(3)

def kitchen():
    time.sleep(random.uniform(0, 3))
    while time.time() < end:
        call("GET kitchen lines", "GET", f"/api/v1/branches/{BR}/kitchen-lines?limit=100", staff)
        sleep(3)

def cashier_reader():
    time.sleep(random.uniform(0, 3))
    while time.time() < end:
        call("GET visit bill (cashier)", "GET", f"/api/v1/visits/{random.choice(ids['open_visits'])}/bill", staff)
        sleep(3)

def orderer():
    time.sleep(random.uniform(0, 1))
    while time.time() < end:
        item, option = random.choice(ids["items"])
        call("POST assisted order", "POST", f"/api/v1/visits/{random.choice(ids['order_visits'])}/orders",
             {**staffw, "Idempotency-Key": str(uuid.uuid4())},
             {"menu_revision": ids["menu_revision"], "lines": [{"item_id": item, "quantity": 1, "option_ids": [option]}]})
        sleep(1)

def joiner():
    time.sleep(random.uniform(0, 1))
    while time.time() < end:
        call("POST assisted queue join", "POST", f"/api/v1/branches/{BR}/queue-tickets",
             {**staffw, "Idempotency-Key": str(uuid.uuid4())}, {"party_size": random.randint(1, 6), "needs": []})
        sleep(1)

def settler(visits):
    time.sleep(random.uniform(0, 2))
    confirm_at = t0 + warm + random.uniform(dur * 0.2, dur * 0.9)
    while time.time() < end and visits:
        v = random.choice(visits)
        st, bill = call("GET visit bill (cashier)", "GET", f"/api/v1/visits/{v}/bill", staff)
        if st != 200 or bill["visit_state"] not in ("open", "settling"):
            visits = [x for x in visits if x != v]
            continue
        if bill["visit_state"] == "open":
            st, bill = call("POST settlement begin", "POST", f"/api/v1/visits/{v}/settlement/begin",
                            {**staffw, "Idempotency-Key": str(uuid.uuid4())}, {"expected_version": bill["bill_version"]})
            if st != 200:
                sleep(1); continue
        if time.time() > confirm_at:
            call("POST settlement confirm", "POST", f"/api/v1/visits/{v}/settlement/confirm",
                 {**staffw, "Idempotency-Key": str(uuid.uuid4())},
                 {"expected_version": bill["bill_version"], "amount_satang": bill["total_satang"], "method": "cash", "verification_note": "qualification"})
            visits = [x for x in visits if x != v]
            confirm_at = time.time() + dur * 0.1
        else:
            call("POST settlement reopen", "POST", f"/api/v1/visits/{v}/settlement/reopen",
                 {**staffw, "Idempotency-Key": str(uuid.uuid4())}, {"expected_version": bill["bill_version"], "reason": "qualification cycle"})
        sleep(1)

def member(tok):
    time.sleep(random.uniform(0, 30))
    h = {"Cookie": "__Host-tf_member=" + tok}
    while time.time() < end:
        call("GET member loyalty", "GET", "/api/v1/members/me/loyalty", h)
        call("GET member loyalty entries", "GET", "/api/v1/members/me/loyalty/entries?limit=25", h)
        sleep(30)

def manager():
    time.sleep(random.uniform(0, 10))
    while time.time() < end:
        call("GET daily report (31 days)", "GET", f"/api/v1/branches/{BR}/reports/daily?from={ids['report_from']}&to={ids['report_to']}", staff)
        call("GET audit events", "GET", f"/api/v1/branches/{BR}/audit-events?from={ids['report_from']}&to={ids['report_to']}&limit=50", staff)
        sleep(60)

threads = [threading.Thread(target=tracker, args=t) for t in ids["trackers"][:150]]
threads += [threading.Thread(target=diner, args=d) for d in ids["diners"][:150]]
threads += [threading.Thread(target=host) for _ in range(10)] + [threading.Thread(target=kitchen) for _ in range(5)]
threads += [threading.Thread(target=cashier_reader) for _ in range(5)]
threads += [threading.Thread(target=orderer) for _ in range(6)] + [threading.Thread(target=joiner) for _ in range(1)]
settle = ids["settle_visits"]
threads += [threading.Thread(target=settler, args=(settle[i::3],)) for i in range(3)]
threads += [threading.Thread(target=member, args=(m,)) for m in ids["members"][:50]] + [threading.Thread(target=manager)]
for t in threads:
    t.daemon = True
    t.start()
for t in threads:
    t.join()

total = unexpected = 0
for route, xs in sorted(samples.items()):
    lat = sorted(m for m, _ in xs)
    codes = {}
    for _, c in xs:
        codes[str(c)] = codes.get(str(c), 0) + 1
    q = statistics.quantiles(lat, n=100) if len(lat) > 1 else [lat[0]] * 99
    total += len(xs)
    unexpected += sum(n for c, n in codes.items() if c == "0" or c.startswith("5"))
    print(json.dumps({"route": route, "samples": len(xs), "rps": round(len(xs) / dur, 2), "codes": codes,
                      "p50_ms": round(q[49], 2), "p95_ms": round(q[94], 2), "p99_ms": round(q[98], 2), "max_ms": round(lat[-1], 2),
                      "avg_bytes": int(statistics.mean(sizes[route]))}))
print(json.dumps({"summary": True, "total_rps": round(total / dur, 1), "requests": total, "unexpected_errors": unexpected,
                  "unexpected_rate": round(unexpected / max(total, 1), 5), "client_exceptions": errors, "warmup_s": warm, "measure_s": dur,
                  "threads": len(threads)}))
