Good question to stop and answer precisely, because "what goes over which stream" is exactly the kind of decision that's easy to get sloppy about once you're deep in implementation — fields end up wherever's convenient rather than where they belong. Let me work through this by first principles, then give you the concrete inventory.

---

## The Governing Principle

Before listing fields, state the rule that should decide where each piece of information belongs, because several things we've built so far already violate it in small ways I should flag:

```
A piece of information belongs on the CONNECTOR ↔ GATEWAY management
stream if and only if it concerns THIS gateway's relationship with
THIS specific connector instance, right now.

A piece of information belongs on the GATEWAY ↔ CP stream if and
only if it concerns something CP needs to know or decide about,
that spans potentially MULTIPLE gateways or requires CP's authority
to resolve (policy, revocation, tenant config, cross-gateway state).

Data that answers "is this specific TCP/QUIC connection healthy
right now" → connector↔gateway
Data that answers "should this connector be allowed to exist
at all, under current organizational policy" → gateway↔CP
```

If you find a field that seems to need both, that's usually a sign it should live in ONE place and get relayed, not duplicated — exactly the SuspendCommand pattern we already built, where CP is authoritative and gateway relays.

---

## Connector ↔ Gateway Management Stream

### Connector → Gateway

```
┌─────────────────────────────────────────────────────────────────┐
│ ConnectorHello (once, at connection start)                       │
├─────────────────────────────────────────────────────────────────┤
│ connector_id       │ who is connecting                          │
│ tenant_id          │ which tenant this connector serves          │
│ token              │ proves current authorization (per our       │
│                     │ earlier mTLS-vs-token discussion)          │
│ version            │ connector binary version — CP/gateway may   │
│                     │ need this to decide if a forced upgrade    │
│                     │ is required                                │
│ apps               │ [{id, subdomain, addr, proto, is_public}]   │
│                     │ what this connector can proxy to           │
│ transport           │ "quic"/"grpc"/"websocket" — which tunnel   │
│                     │ transport this connector negotiated,       │
│                     │ purely informational for gateway's own     │
│                     │ logging/metrics, NOT used for auth         │
│                     │ decisions (auth already happened via mTLS  │
│                     │ + token before this message is even read)  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ConnectorHeartbeat (every 30s)                                    │
├─────────────────────────────────────────────────────────────────┤
│ seq                │ monotonic counter — gateway detects gaps    │
│                     │ (missed heartbeats) by comparing to        │
│                     │ last-seen seq, not just by timing alone    │
│ tunnel_state       │ "connected"/"disconnected" — is the DATA     │
│                     │ plane (QUIC/etc) currently attached from    │
│                     │ the connector's own point of view — this   │
│                     │ matters because the connector can detect   │
│                     │ its own tunnel dying faster than gateway's │
│                     │ heartbeat-staleness timeout would notice   │
│ tunnel_transport   │ current active transport, in case it        │
│                     │ changed since Hello (e.g. QUIC dropped,     │
│                     │ fell back to gRPC mid-session — gateway     │
│                     │ should know this happened for observability│
│ active_streams     │ how many requests THIS connector currently  │
│                     │ believes it has open — cross-checked        │
│                     │ against gateway's own registry count from  │
│                     │ StreamInfo tracking; a mismatch here is a   │
│                     │ real signal something is wrong (leaked      │
│                     │ stream tracking on one side or the other)  │
│ uptime_seconds     │ how long this connector process has been     │
│                     │ running — helps distinguish "just restarted"│
│                     │ from "long-running, something's degrading"  │
└─────────────────────────────────────────────────────────────────┘
```

### Gateway → Connector

```
┌─────────────────────────────────────────────────────────────────┐
│ ConnectorHelloAck (once, response to Hello)                       │
├─────────────────────────────────────────────────────────────────┤
│ session_id         │ correlates this specific connection instance│
│                     │ for logging/audit — a connector reconnecting│
│                     │ gets a NEW session_id each time, which is   │
│                     │ how you'd detect flapping in CP's logs      │
│ server_version     │ gateway binary version — connector could     │
│                     │ log a warning if there's a known             │
│                     │ compatibility issue, though you don't have  │
│                     │ that logic built yet                        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ SuspendCommand / ResumeCommand (pushed as CP decides, relayed)     │
├─────────────────────────────────────────────────────────────────┤
│ connector_id       │ (added per the gap flagged last time)        │
│ reason             │ human-readable, for connector-side logging   │
│                     │ AND so an IT person checking their own      │
│                     │ tray icon / CLI status can see WHY they're  │
│                     │ suspended, not just THAT they are           │
│ operator           │ which CP operator/system issued this — for   │
│                     │ audit trail visible even at the connector    │
│ signature           │ CP signature — connector verifies with        │
│                     │ embedded CP public key before acting, per   │
│                     │ your PKI architecture document's core rule  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ RejectMessage (if gateway refuses this connector entirely)         │
├─────────────────────────────────────────────────────────────────┤
│ reason             │ e.g. "token_expired", "cert_revoked"          │
│ code               │ machine-readable — lets connector distinguish│
│                     │ "retry later" from "re-register required"    │
│ permanent          │ bool — connector decides whether to loop      │
│                     │ reconnecting or exit and prompt re-registration│
└─────────────────────────────────────────────────────────────────┘
```

**One thing I want to flag as a real gap right now, not later:** notice that **RotationCommand and policy/trust/CRL bundle pushes were designed for the gateway↔CP relationship in your PKI document, but nothing analogous exists for connector↔gateway.** Does a connector ever need a policy bundle? Right now, no — the gateway does all authorization decisions (once we actually build that missing piece), and the connector is a dumb proxy. That's a deliberate, correct design choice — **the connector should know as little as possible**, which is good for your security model. I'm naming this explicitly so it's a decision you've consciously made, not an oversight: connectors get identity commands (suspend/resume, rotation) but never policy content, because policy enforcement never happens at the connector.

---

## Gateway ↔ CP Stream

### Gateway → CP

```
┌─────────────────────────────────────────────────────────────────┐
│ HelloMessage (once, at gateway startup)                           │
├─────────────────────────────────────────────────────────────────┤
│ gateway_id         │ stable identity                              │
│ version             │ gateway binary version                       │
│ system_info         │ {os, cpu_count, memory_bytes} — CP can use   │
│                     │ this for capacity planning across the fleet │
│                     │ of gateways, or alerting if a gateway is on  │
│                     │ unusually constrained hardware               │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ HeartbeatMessage (every 30s)                                       │
├─────────────────────────────────────────────────────────────────┤
│ seq                 │ same gap-detection purpose as connector's    │
│ active_connectors   │ registry.Count() — how many connectors THIS │
│                     │ gateway currently serves — CP aggregates    │
│                     │ this across all gateways for a fleet-wide   │
│                     │ view                                          │
│ active_streams      │ SUM of ActiveStreamCount() across all         │
│                     │ registry entries — total in-flight requests │
│                     │ this gateway is currently proxying            │
│ state                │ "ACTIVE"/"DRAINING" — matters for rolling    │
│                     │ upgrades/deploys, so CP (or a load balancer  │
│                     │ config CP manages) knows not to route new    │
│                     │ traffic here                                  │
│ resource_usage      │ {cpu_percent, memory_bytes, goroutines} —    │
│                     │ operational health signal                     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ AccessLogBatch (every 10s, batched)                                 │
├─────────────────────────────────────────────────────────────────┤
│ Every proxied request: request_id, tenant_id, connector_id,        │
│ user_id, user_email, app_id, method, path, status_code,             │
│ latency_ms, decision (ALLOW/DENY), deny_reason, source_ip           │
│                                                                       │
│ This is THE audit trail. This is what makes Ashrix sellable to an   │
│ enterprise security team — "show me every access to this app,       │
│ by whom, when." Notice this is the field set that becomes           │
│ meaningful only once the still-missing authorization check exists —│
│ right now "decision" would always be ALLOW because nothing ever    │
│ actually evaluates a DENY. That's the same gap I've flagged         │
│ repeatedly, showing up again here in a new place: your audit log   │
│ schema already anticipates authorization decisions that the code   │
│ doesn't make yet.                                                    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ AnomalyReport (as detected)                                         │
├─────────────────────────────────────────────────────────────────┤
│ anomaly_type        │ e.g. STREAM_FLOOD (from your earlier          │
│                     │ vulnerability list — a connector opening      │
│                     │ far more streams than normal), INVALID_TOKEN, │
│                     │ CERT_MISMATCH                                  │
│ connector_id        │ which connector triggered this, if applicable │
│ severity             │ lets CP decide alert urgency                  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ CSR + possession proof (for gateway's OWN cert renewal)             │
├─────────────────────────────────────────────────────────────────┤
│ Exactly as built in your PKI/rotator work earlier — this is a       │
│ separate unary RPC in your design (RenewCert), not a stream         │
│ message, per the design decision made when we discussed PKIInfra.   │
└─────────────────────────────────────────────────────────────────┘
```

### CP → Gateway

```
┌─────────────────────────────────────────────────────────────────┐
│ HelloAck (once)                                                     │
├─────────────────────────────────────────────────────────────────┤
│ server_version                                                       │
│ needs_policy/needs_trust/needs_crl │ per our bundle-sync              │
│                                       discussion — though we           │
│                                       concluded these should likely   │
│                                       always be true given the        │
│                                       cold_start reasoning, so this   │
│                                       field may be redundant now —    │
│                                       worth revisiting once you       │
│                                       actually build bundle sync      │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ PolicyBundle / TrustBundle / CRLUpdate                               │
├─────────────────────────────────────────────────────────────────┤
│ Exactly as specified in your PKI document — version, signed          │
│ payload, max_valid_until. THIS is where the actual authorization     │
│ rules that the still-missing httpproxy check needs to come from.    │
│ Right now these messages are defined in proto but there is no        │
│ code anywhere that RECEIVES one, parses it into an in-memory         │
│ policy structure, and consults that structure in ServeHTTP. That's  │
│ the concrete, specific next piece of work this inventory exposes.    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ SuspendCommand / ResumeCommand / RotationCommand / DrainCommand      │
├─────────────────────────────────────────────────────────────────┤
│ As built. Note SuspendCommand/ResumeCommand here target a            │
│ CONNECTOR (relayed onward) — but there should ALSO be a gateway-     │
│ level suspend, distinct from connector-level, for when CP wants     │
│ to pull an entire gateway node out of rotation (maintenance,         │
│ suspected compromise of the gateway itself). This doesn't exist      │
│ in what we've built — worth deciding if you need it now or later.    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ RenewCertResponse                                                     │
├─────────────────────────────────────────────────────────────────┤
│ certificate_pem, expires_at — as built                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## The Two Real Gaps This Inventory Just Exposed

I want to be precise that going through this exercise surfaced two concrete, actionable things, not just an organizational list:

**First — the policy bundle plumbing does not exist yet, and this is now unambiguous.** You have the proto message shape (`PolicyBundle`), you have the design principle ("CP creates trust, gateway distributes"), but there is no `handlePolicyBundle` function anywhere in the gateway code we've written that takes an incoming `PolicyBundle`, deserializes its `payload` bytes into something queryable, and stores it somewhere `httpproxy.ServeHTTP` can check. This is the concrete missing link between "CP has policy" and "gateway enforces policy" — and it's the actual mechanism the repeatedly-flagged authorization gap needs to be built on top of.

**Second — gateway-level suspend versus connector-level suspend are conflated right now.** Your entire SuspendCommand design, as built, only ever targets a specific `connector_id`. If CP needs to pull an entire gateway offline — suspected compromise, planned maintenance, decommissioning — there's no message for that. It's a smaller gap than the first one, but it's asymmetric with your PKI document's own state machine, which explicitly gives both Gateway and Connector the same lifecycle including REVOKED. Worth a deliberate decision: do you need gateway-level suspend now, or is "revoke the gateway's cert" (which you already have) sufficient for that case, making a separate suspend command redundant? I'd lean toward the latter — revocation already gives you the hard stop — but name it as a decision rather than leaving it unconsidered.