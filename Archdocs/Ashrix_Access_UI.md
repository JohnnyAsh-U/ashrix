## Ashrix Access — Interface UI Specification

---

## Design Principles

```
Simple over clever.
Every screen answers one question without requiring the user to dig.
The IT admin deploys a gateway in under 5 minutes.
The end user reaches their app in under 30 seconds.
The security admin revokes a session in under 10 seconds.
```

Three users. Three mental models.

```
IT Admin:        "Are my gateways up? Are my apps published?"
Security Admin:  "Who has access? Who is currently in?"
Auditor:         "What happened? Who did what?"
```

One constraint that shapes everything:
The dashboard is a Next.js web app served from Ashrix SaaS.
The access portal is server-rendered HTML served by the gateway binary.
They are different surfaces for different audiences.

---

## Surface 1: Admin Dashboard
## (dashboard.ashrix.io — Next.js, admin-facing)

---

### Sidebar

Always visible. Collapsible to icon-only on narrow screens.
Active page: indigo left accent bar.
Badge counts update via SSE — no refresh.

```
┌──────────────────────────────┐
│  ▪ Ashrix Access    [Acme ▾] │  ← org switcher (future multi-org)
├──────────────────────────────┤
│                              │
│  🏠  Overview                │
│                              │
│  INFRASTRUCTURE              │
│  ⬡   Applications            │
│  ⟳   Connectors              │
│  ◈   Gateways                │
│                              │
│  ACCESS CONTROL              │
│  ⊞   Policies                │
│  👥  Users                   │
│  ⚡   Sessions             3  │ ← active alert count
│                              │
│  LOGS                        │
│  📋  Access Log              │
│  📝  Audit Log               │
│                              │
│  SETTINGS                    │
│  ⚙   Organisation            │
│  🔌  Identity (IdP)          │
│  👤  Admin Users             │
│                              │
├──────────────────────────────┤
│  james@acme.com    [Logout]  │
└──────────────────────────────┘
```

Bottom of sidebar (always visible, even when collapsed):
```
● 2 gateways  ● 12 apps  ● 61 sessions
```
Any dot goes red if a gateway is offline or a cert is expiring.
One glance. No page navigation required to know if something is wrong.

---

### Page 1 — Overview

Purpose: Health at a glance. No digging.
First page every admin sees on login.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Good morning, James.                       Tuesday 20 May · 09:14  │
│  Acme Corp                                                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌──────┐ │
│  │ Applications   │ │ Active Users   │ │ Active Sessions│ │Denied│ │
│  │      12        │ │      247       │ │       61       │ │  18  │ │
│  │  12 healthy    │ │  last 24h      │ │   right now    │ │today │ │
│  └────────────────┘ └────────────────┘ └────────────────┘ └──────┘ │
│                                                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ALERTS                                               [View all →]  │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ⚠  Connector "erp-connector" disconnected         6 min ago  │  │
│  │     ERP is unreachable. Users will see offline page.          │  │
│  │     [Go to connector →]                                       │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  No other alerts.                                                    │
│                                                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  GATEWAYS                                         [Manage →]        │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ●  London HQ      healthy    9ms    8 apps    47 sessions    │  │
│  │  ●  Manchester     healthy   18ms    4 apps    14 sessions    │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  APPLICATIONS                                     [Manage →]        │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ✓  CRM          ✓  Grafana     ✓  GitLab      ✓  HR Portal  │  │
│  │  ✓  Wiki         ✓  Jenkins     ✗  ERP         ✓  Jira       │  │
│  │     (8 more)                                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  ✗ ERP is offline. [Investigate →]                                   │
│                                                                      │
│  TRAFFIC  (last 24h)              RECENT ACCESS EVENTS    (live)    │
│  ┌───────────────────────┐        ┌───────────────────────────────┐ │
│  │  [Line chart          │        │  09:14 alice@  CRM    allowed │ │
│  │   requests per hour   │        │  09:13 bob@    Finance allowed│ │
│  │   allow vs deny]      │        │  09:11 carol@  DevEnv denied  │ │
│  │                       │        │  09:11 carol@  DevEnv denied  │ │
│  │   Today: 8,420 req    │        │  09:09 dave@   GitLab allowed │ │
│  │   Denied: 18 (0.2%)   │        │  09:08 alice@  Grafana allowed│ │
│  └───────────────────────┘        └───────────────────────────────┘ │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

Live behaviour (SSE):
- Gateway status dots: update every 30s
- Alert strip: appears immediately on any gateway/connector event
- Recent events: stream in as they happen
- Stat counters: refresh every 60s

Alert strip rules:
- Gateway offline → red banner, persists until resolved
- Connector disconnected → orange banner
- TLS cert expiring in <7 days → yellow banner
- No alerts → "No alerts. Everything looks good." (not blank)

---

### Page 2 — Applications

Purpose: Every protected app. Add, monitor, manage policies.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Applications                               [+ Add Application]     │
├─────────────────────────────────────────────────────────────────────┤
│  Search...         Filter: All  HTTP  SSH  TCP  Healthy  Offline    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  NAME           HOSTNAME                  STATUS   POLICIES  USERS  │
│  ─────────────────────────────────────────────────────────────────  │
│  CRM            crm.acme.com              ✓ up     2 rules   34     │
│  Grafana        grafana.acme.com          ✓ up     1 rule    18     │
│  GitLab         git.acme.com              ✓ up     2 rules   41     │
│  HR Portal      hr.acme.com               ✓ up     1 rule    12     │
│  Jenkins        ci.acme.com               ✓ up     1 rule     6     │
│  ERP            erp.acme.com              ✗ offline 1 rule    0     │
│  Wiki           wiki.acme.com             ✓ up     1 rule   147     │
│  Jump Server    ssh.acme.com   [SSH]      ✓ up     1 rule     2     │
│                                                                      │
│  Click any row to expand details.                                    │
└─────────────────────────────────────────────────────────────────────┘
```

App row expanded (click to expand inline, not a new page):

```
┌─────────────────────────────────────────────────────────────────────┐
│  ▼  CRM                                                             │
│  ─────────────────────────────────────────────────────────────────  │
│  Hostname        crm.acme.com                                        │
│  Upstream        10.0.1.10:8080          Protocol   HTTP            │
│  Gateway         London HQ                                           │
│  TLS cert        ✓ valid · expires Aug 7 (79 days)                  │
│  Connector       ✓ crm-connector · last check 18s ago               │
│  Visibility      🔒 Private (requires login)                         │
│                                                                      │
│  POLICIES                                [+ Add rule]               │
│  ✓  Engineering → allow   (priority 10)              [Edit] [···]   │
│  ✓  HR Team → allow       (priority 20)              [Edit] [···]   │
│  ─  Default: DENY ALL                                                │
│                                                                      │
│  TRAFFIC (last 1h)                                                   │
│  847 requests · avg 38ms · 0 denied                                  │
│                                                                      │
│  RATE LIMITING   100 req/min per user    [Edit]                      │
│  SECURITY HEADERS  ✓ enabled             [View headers]             │
│                                                                      │
│  [Edit App]  [View Access Log]  [Delete App]                         │
└─────────────────────────────────────────────────────────────────────┘
```

ERP row expanded (offline state):

```
┌─────────────────────────────────────────────────────────────────────┐
│  ▼  ERP                                                             │
│  ─────────────────────────────────────────────────────────────────  │
│  Hostname        erp.acme.com                                        │
│  Upstream        10.0.1.40:8443          Protocol   HTTP            │
│  Gateway         London HQ                                           │
│  TLS cert        ✓ valid · expires Aug 7 (79 days)                  │
│                                                                      │
│  ✗  Connector    erp-connector · DISCONNECTED since 09:08            │
│     Users reaching erp.acme.com see "Application unavailable."      │
│     Internal IP and error details are never shown to users.          │
│     [Go to connector →]  [View connector logs →]                    │
│                                                                      │
│  [Edit App]  [View Access Log]  [Delete App]                         │
└─────────────────────────────────────────────────────────────────────┘
```

---

Add Application (slide-over panel from the right):

```
┌────────────────────────────────────────┐
│  Add Application                  [×]  │
├────────────────────────────────────────┤
│                                        │
│  Name                                  │
│  [ CRM                             ]   │
│                                        │
│  Public hostname                       │
│  [ crm            ].acme.com           │
│  ↳ Certificate will be auto-issued     │
│                                        │
│  Internal upstream URL                 │
│  [ http://10.0.1.10:8080           ]   │
│                                        │
│  Protocol                              │
│  ◉ HTTP / HTTPS                        │
│  ○ TCP                                 │
│  ○ SSH                                 │
│                                        │
│  Gateway                               │
│  [ London HQ                  ▾    ]   │
│                                        │
│  Visibility                            │
│  ◉ Private  (requires login)           │
│  ○ Public   (no login required)        │
│                                        │
│  Rate limiting                         │
│  ◉ 100 requests / minute / user        │
│  ○ Custom  [      ] req / [    ] per   │
│  ○ None                                │
│                                        │
│  ┌────────────────────────────────┐    │
│  │ ℹ  Default policy is DENY ALL. │    │
│  │    You will add access rules   │    │
│  │    after saving.               │    │
│  └────────────────────────────────┘    │
│                                        │
│  [Cancel]           [Add Application]  │
└────────────────────────────────────────┘
```

Public application confirmation (shown when public is selected):

```
┌────────────────────────────────────────┐
│  ⚠  Public application                 │
│                                        │
│  This application will be accessible   │
│  without login. Anyone on the internet │
│  can reach it.                         │
│                                        │
│  Ashrix will still provide:            │
│  ✓ TLS termination                     │
│  ✓ Rate limiting                       │
│  ✓ Security headers                    │
│  ✓ Access logging                      │
│                                        │
│  Authentication: NONE                  │
│                                        │
│  [ Cancel ]    [ I understand, continue]│
└────────────────────────────────────────┘
```

---

### Page 3 — Policies

Purpose: Create and manage access rules.
         The most important admin workflow.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Policies                                    [+ Create Rule]        │
├─────────────────────────────────────────────────────────────────────┤
│  Filter by app: [ All apps ▾ ]    Status: All  Active  Disabled     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  RULE NAME                  APP          WHO           STATUS        │
│  ─────────────────────────────────────────────────────────────────  │
│  Engineering → CRM          CRM          engineering   ✓ active      │
│  HR → CRM                   CRM          hr-team       ✓ active      │
│  Engineering → Grafana      Grafana      engineering   ✓ active      │
│  Engineering → GitLab       GitLab       engineering   ✓ active      │
│  Contractors → GitLab       GitLab       contractors   ✓ active      │
│  All Staff → Wiki           Wiki         @acme.com     ✓ active      │
│  SRE → Jump Server          Jump Server  sre           ✓ active      │
│                                                                      │
│  Click any rule to view · edit · version history · disable · delete  │
└─────────────────────────────────────────────────────────────────────┘
```

Create Rule — full page form (not a modal, rules are important):

```
┌─────────────────────────────────────────────────────────────────────┐
│  ←  Policies          Create Access Rule                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Rule name                                                           │
│  [ Engineering team access to CRM                              ]     │
│                                                                      │
│  ── APPLICATION ──────────────────────────────────────────────────  │
│  [ CRM (crm.acme.com)                                      ▾    ]   │
│                                                                      │
│  ── WHO CAN ACCESS ───────────────────────────────────────────────  │
│                                                                      │
│  ◉ By group (from your IdP)                                          │
│    [ engineering ×]  [ devops ×]    [+ Add group]                   │
│    34 users · 8 users                                                │
│                                                                      │
│  ○ By email address                                                  │
│    [+ Add email]                                                     │
│                                                                      │
│  ○ By domain                                                         │
│    [+ Add domain]  e.g. company.com                                  │
│                                                                      │
│  ── NETWORK RESTRICTIONS  (optional) ────────────────────────────   │
│  ○ No restriction  (any IP)                                          │
│  ◉ Allow only these IP ranges:                                       │
│    [ 196.10.0.0/16  ×]  [+ Add range]                               │
│    (leave empty to allow any IP)                                     │
│                                                                      │
│  Block these IP ranges:                                              │
│  [+ Add blocked range]                                               │
│                                                                      │
│  ── MFA REQUIREMENT ──────────────────────────────────────────────  │
│  ○ Not required                                                      │
│  ◉ Required  (IdP must confirm MFA was used)                         │
│                                                                      │
│  ── PRIORITY ─────────────────────────────────────────────────────  │
│  [ 10 ]   Lower number = evaluated first                             │
│           Deny rules should have lower priority than allow rules.    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  PREVIEW                                                    │    │
│  │  Engineering and DevOps (42 users) can access CRM           │    │
│  │  from IPs in 196.10.0.0/16, with MFA required.              │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  [Save as Draft]                          [Activate Rule]            │
└─────────────────────────────────────────────────────────────────────┘
```

Rule version history (slide-over from right):

```
┌──────────────────────────────────────┐
│  Version history                [×]  │
│  Engineering → CRM                   │
├──────────────────────────────────────┤
│                                      │
│  v3  Today 09:12    james@           │
│      Added devops group              │
│      [Rollback to v2]                │
│                                      │
│  v2  May 18 14:33   james@           │
│      Added IP restriction            │
│      [Rollback to v1]                │
│                                      │
│  v1  May 10 09:00   james@           │
│      Rule created                    │
│                                      │
└──────────────────────────────────────┘
```

---

### Page 4 — Connectors

Purpose: Every connector enrolled. Health, revoke, re-enroll.
         The operational side of the killer feature.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Connectors                         [+ Generate Enrollment Token]   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  NAME              APP          GATEWAY     STATUS      LAST SEEN   │
│  ─────────────────────────────────────────────────────────────────  │
│  crm-connector     CRM          London HQ   ✓ connected  18s ago    │
│  grafana-connector Grafana      London HQ   ✓ connected  22s ago    │
│  gitlab-connector  GitLab       London HQ   ✓ connected  31s ago    │
│  hr-connector      HR Portal    London HQ   ✓ connected  44s ago    │
│  erp-connector     ERP          London HQ   ✗ disconnected 6m ago   │
│  wiki-connector    Wiki         Manchester  ✓ connected  12s ago    │
│                                                                      │
│  Each row: [View details]  [Revoke]  [Re-enroll]                    │
└─────────────────────────────────────────────────────────────────────┘
```

Connector row expanded:

```
┌─────────────────────────────────────────────────────────────────────┐
│  ▼  erp-connector                                                   │
│  ─────────────────────────────────────────────────────────────────  │
│  App             ERP (erp.acme.com)                                  │
│  Gateway         London HQ                                           │
│  Upstream        10.0.1.40:8443                                      │
│  Status          ✗ Disconnected since 09:08 (6 minutes ago)         │
│  Last connected  Today 09:08                                         │
│  Enrolled        May 10, 2026 by james@acme.com                     │
│                                                                      │
│  ⚠  Users reaching erp.acme.com are seeing                          │
│     "Application unavailable."                                       │
│                                                                      │
│  TO RESTORE: run the connector on the ERP server:                    │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ./ashrix-connector start                                     │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  If the connector binary was removed, re-enroll:                     │
│  [Generate new enrollment token →]                                   │
│                                                                      │
│  [Revoke connector]                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

Generate Enrollment Token (slide-over):

```
┌────────────────────────────────────────┐
│  Generate Connector Token         [×]  │
├────────────────────────────────────────┤
│                                        │
│  Connector name                        │
│  [ crm-connector-2                 ]   │
│                                        │
│  Application                           │
│  [ CRM                         ▾   ]   │
│                                        │
│  Gateway                               │
│  [ London HQ                   ▾   ]   │
│                                        │
│  Token expires in                      │
│  ◉ 1 hour   ○ 4 hours                  │
│                                        │
│  [Generate Token]                      │
│                                        │
│  ── After generation ────────────────  │
│                                        │
│  Run this on the app server:           │
│  ┌──────────────────────────────────┐  │
│  │  ashrix-connector enroll \       │  │
│  │    --token=conn_9xk2m7p3 \       │  │
│  │    --gateway=gw.acme.com \       │  │
│  │    --upstream=10.0.1.10:8080 \   │  │
│  │    --app=crm                     │  │
│  └──────────────────────────────────┘  │
│  [Copy command]  Expires in: 59:43     │
│                                        │
│  ⏳ Waiting for connector to connect…  │
│  Token is one-time use.                │
│  Invalidated immediately on use.       │
│                                        │
└────────────────────────────────────────┘
```

---

### Page 5 — Gateways

Purpose: Gateway fleet. Enroll, monitor, decommission.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Gateways                                   [+ Enroll Gateway]      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ●  LONDON HQ                                    [Manage ▾]   │  │
│  │  ─────────────────────────────────────────────────────────    │  │
│  │  Status       ✓ healthy              Version    v1.0.0        │  │
│  │  Latency      9ms (control plane)    Uptime     12d 4h        │  │
│  │  Apps         8 connected            Sessions   47 active     │  │
│  │  Last sync    4s ago                 Cert       79 days       │  │
│  │                                                               │  │
│  │  App health:                                                  │  │
│  │  CRM ✓  Grafana ✓  GitLab ✓  HR Portal ✓  ERP ✗  (+3 more)  │  │
│  │                                                               │  │
│  │  [View logs]  [Decommission]                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ●  MANCHESTER                                   [Manage ▾]   │  │
│  │  ─────────────────────────────────────────────────────────    │  │
│  │  Status       ✓ healthy              Version    v1.0.0        │  │
│  │  Latency      18ms (control plane)   Uptime     12d 4h        │  │
│  │  Apps         4 connected            Sessions   14 active     │  │
│  │  Last sync    7s ago                 Cert       79 days       │  │
│  │                                                               │  │
│  │  App health:  Wiki ✓  Payroll ✓  SQL ✓  VPN-mgmt ✓           │  │
│  │                                                               │  │
│  │  [View logs]  [Decommission]                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

Enroll Gateway (slide-over):

```
┌────────────────────────────────────────┐
│  Enroll Gateway                   [×]  │
├────────────────────────────────────────┤
│                                        │
│  Gateway name                          │
│  [ London HQ                       ]   │
│                                        │
│  Platform                              │
│  ◉ Linux binary   ○ Docker             │
│  ○ Windows        ○ Kubernetes         │
│                                        │
│  [Generate Enrollment Token]           │
│                                        │
│  ── After generation ────────────────  │
│                                        │
│  1. Download the binary:               │
│  ┌──────────────────────────────────┐  │
│  │  curl -L https://dl.ashrix.io/   │  │
│  │  gateway/linux-amd64 \           │  │
│  │  -o ashrix-gateway && chmod +x   │  │
│  │  ashrix-gateway                  │  │
│  └──────────────────────────────────┘  │
│  [Copy]                                │
│                                        │
│  2. Enroll and start:                  │
│  ┌──────────────────────────────────┐  │
│  │  ./ashrix-gateway enroll \       │  │
│  │    --token=enroll_7f3kq9x2m4p \  │  │
│  │    --name="London HQ"            │  │
│  │                                  │  │
│  │  ./ashrix-gateway start          │  │
│  └──────────────────────────────────┘  │
│  [Copy]   Token expires in: 14:21      │
│                                        │
│  ⏳ Waiting for gateway to connect…    │
│  (turns green when gateway appears)    │
│                                        │
└────────────────────────────────────────┘
```

Waiting state resolves to:

```
  ✓  London HQ connected successfully.
     Version: v1.0.0 · 0 apps · 0 sessions
     [Go to gateway →]
```

---

### Page 6 — Users

Purpose: Read-only IdP mirror.
         Who has access to what. Active sessions per user.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Users                                                               │
│  Connected IdP: Google Workspace (acme.com) · 247 users · 12 groups │
│  Last synced: 3 minutes ago                          [Sync now]      │
├─────────────────────────────────────────────────────────────────────┤
│  [Users]  [Groups]                                                   │
├─────────────────────────────────────────────────────────────────────┤
│  Search users...                                                     │
│                                                                      │
│  NAME           EMAIL                GROUPS           LAST SEEN     │
│  ─────────────────────────────────────────────────────────────────  │
│  Alice Chen     alice@acme.com       engineering, hr  2 min ago     │
│  Bob Kumar      bob@acme.com         finance          1 hr ago      │
│  Carol Smith    carol@acme.com       engineering      3 hr ago      │
│  David Jones    david@acme.com       sre, it          Online now    │
│  Emma Wilson    emma@acme.com        hr               Today 08:44   │
│                                                                      │
│  Click user → access map, active sessions, recent events            │
└─────────────────────────────────────────────────────────────────────┘
```

User detail page (click user row → navigates to /users/alice):

```
┌─────────────────────────────────────────────────────────────────────┐
│  ←  Users             Alice Chen                                    │
│                        alice@acme.com  ·  engineering, hr           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  PERMITTED APPLICATIONS                                              │
│  ✓  CRM           via: Engineering → CRM rule                       │
│  ✓  Grafana       via: Engineering → Grafana rule                   │
│  ✓  GitLab        via: Engineering → GitLab rule                    │
│  ✓  HR Portal     via: HR → HR Portal rule                          │
│  ✓  Wiki          via: All Staff → Wiki rule                        │
│  ✗  ERP           No matching policy rule                            │
│  ✗  Jenkins       No matching policy rule                            │
│                                                                      │
│  ACTIVE SESSIONS  (1)                                                │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Chrome · macOS  ·  196.10.45.23  ·  London                  │  │
│  │  Gateway: London HQ  ·  Started: today 09:03  ·  8h TTL      │  │
│  │  [Revoke this session]   [Revoke all sessions]               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  RECENT ACCESS  (last 7 days)               [View full log →]       │
│  Today  09:14   CRM          allowed    Chrome · macOS              │
│  Today  09:13   Grafana      allowed    Chrome · macOS              │
│  Today  09:03   HR Portal    allowed    Chrome · macOS              │
│  May 19 14:21   ERP          denied     no matching policy          │
│  May 19 14:21   ERP          denied     no matching policy          │
│  May 19 09:11   CRM          allowed    Chrome · macOS              │
└─────────────────────────────────────────────────────────────────────┘
```

Session revocation confirmation:

```
┌────────────────────────────────────────┐
│  Revoke session?                       │
│                                        │
│  User:     alice@acme.com              │
│  Device:   Chrome · macOS              │
│  Started:  today 09:03                 │
│                                        │
│  Alice will be logged out on her       │
│  next request (typically within 1s).   │
│                                        │
│  [Cancel]        [Revoke session]      │
└────────────────────────────────────────┘
```

---

### Page 7 — Sessions

Purpose: All active sessions across all users.
         The security admin's real-time view.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Active Sessions                                         61 active   │
├─────────────────────────────────────────────────────────────────────┤
│  Filter: User [ all ▾ ]   App [ all ▾ ]   Gateway [ all ▾ ]         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  USER               GATEWAY       IP              STARTED   ACTION  │
│  ─────────────────────────────────────────────────────────────────  │
│  alice@acme.com     London HQ     196.10.45.23    09:03     [×]     │
│  bob@acme.com       London HQ     196.10.45.24    08:47     [×]     │
│  carol@acme.com     London HQ     196.10.45.31    09:11     [×]     │
│  david@acme.com     Manchester    41.200.18.4     08:30     [×]     │
│  emma@acme.com      London HQ     196.10.45.29    08:44     [×]     │
│  (56 more)                                                           │
│                                                                      │
│  [×] = revoke session (with confirmation)                            │
│  Click row = user detail page                                        │
└─────────────────────────────────────────────────────────────────────┘
```

---

### Page 8 — Access Log

Purpose: Every proxied request. Full search and filter.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Access Log                                [Export JSON]  [Export CSV│
├─────────────────────────────────────────────────────────────────────┤
│  User [ all ▾ ]  App [ all ▾ ]  Result [ all ▾ ]  Date [ today ▾ ]  │
│                                                   [Search path... ]  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  TIME     USER          APP        METHOD  STATUS  LATENCY  RESULT  │
│  ─────────────────────────────────────────────────────────────────  │
│  09:14:33 alice@        CRM        GET     200     38ms     allowed │
│  09:14:21 alice@        CRM        POST    201     44ms     allowed │
│  09:13:57 bob@          Finance    GET     200     29ms     allowed │
│  09:11:44 carol@        ERP        GET     —       —        denied  │
│  09:11:44 carol@        ERP        GET     —       —        denied  │
│  09:09:12 dave@         GitLab     GET     200     67ms     allowed │
│  09:08:03 unknown       ERP        —       —       —        denied  │
│                                                                      │
│  Click any row for full event detail.                                │
└─────────────────────────────────────────────────────────────────────┘
```

Row expanded (click):

```
┌─────────────────────────────────────────────────────────────────────┐
│  ▼  09:11:44  carol@acme.com  →  ERP                                │
│  ─────────────────────────────────────────────────────────────────  │
│  Result       DENIED                                                 │
│  Reason       no_matching_policy                                     │
│  User         carol@acme.com                                         │
│  Groups       engineering                                            │
│  App          ERP (erp.acme.com)                                     │
│  Method       GET                                                    │
│  Path         /dashboard                                             │
│  IP           196.10.45.31                                           │
│  Gateway      London HQ                                              │
│  Session      sess_carol_abc123                                      │
│  Timestamp    2026-05-20T09:11:44Z                                   │
│                                                                      │
│  [View user carol →]   [Add policy for ERP →]                       │
└─────────────────────────────────────────────────────────────────────┘
```

The "Add policy for ERP" shortcut is context-aware.
When a denied request has reason `no_matching_policy`,
the shortcut pre-fills the Create Rule form with
the app already selected and the user's groups suggested.
Reduces time-to-fix from 5 minutes to 30 seconds.

---

### Page 9 — Audit Log

Purpose: Admin actions. Append-only. Exportable.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Audit Log                            [Export JSON]  [Export CSV]   │
├─────────────────────────────────────────────────────────────────────┤
│  Actor [ all ▾ ]  Action [ all ▾ ]  Date [ last 7 days ▾ ]          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  TIME     ACTOR          ACTION             TARGET                   │
│  ─────────────────────────────────────────────────────────────────  │
│  09:12    james@         policy.updated     Engineering → CRM        │
│  09:00    james@         app.created        ERP                      │
│  May 19   sarah@         session.revoked    carol@acme.com           │
│  May 19   james@         policy.created     All Staff → Wiki         │
│  May 18   james@         connector.enrolled erp-connector            │
│  May 18   james@         gateway.enrolled   London HQ                │
│  May 10   james@         org.created        Acme Corp                │
│                                                                      │
│  Audit log is append-only. Entries cannot be deleted.                │
│  Click any row for full event detail.                                │
└─────────────────────────────────────────────────────────────────────┘
```

---

### Page 10 — Settings: Organisation

```
┌─────────────────────────────────────────────────────────────────────┐
│  Settings                                                            │
│                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ORGANISATION                                                        │
│  Name              [ Acme Corp                             ]        │
│  Slug              [ acme                                  ]        │
│  Portal URL        [ access         ].acme.com                      │
│  Plan              Growth ($149/month)               [Upgrade →]    │
│                                                                      │
│  SESSION SETTINGS                                                    │
│  Session TTL       [ 8   ] hours  (1 to 720)                        │
│  Max sessions/user [ unlimited ▾ ]                                  │
│  After TTL:        ◉ Silent re-auth (if IdP session valid)          │
│                    ○ Force re-login                                  │
│                                                                      │
│  ALERT EMAIL                                                         │
│  [ james@acme.com                              ]                    │
│  Alerts for: gateway offline, cert expiry, connector down           │
│                                                                      │
│  [Save changes]                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

### Page 11 — Settings: Identity (IdP)

```
┌─────────────────────────────────────────────────────────────────────┐
│  Settings                                                            │
│                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  IDENTITY PROVIDER                                                   │
│  Provider          ◉ Google Workspace                               │
│                    ○ Okta                                            │
│                    ○ Microsoft Entra ID                              │
│                    ○ Other OIDC provider                             │
│                                                                      │
│  Client ID         [ 123456789-abc.apps.googleusercontent.com   ]   │
│  Client Secret     [ ••••••••••••••••••••••••    ] [Show] [Change]  │
│  Allowed domain    [ acme.com                                   ]   │
│  ↳ Only users from this domain can authenticate.                    │
│    Leave blank to allow any authenticated user.                     │
│                                                                      │
│  TEST CONNECTION                                                     │
│  [Test IdP connection]                                              │
│  ✓  Connection verified. User sync working. 247 users visible.      │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  ℹ  Ashrix never stores passwords. Users authenticate        │   │
│  │     directly with Google. Ashrix receives only their         │   │
│  │     email address and group memberships.                     │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  [Save changes]                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

### Page 12 — Settings: Admin Users

```
┌─────────────────────────────────────────────────────────────────────┐
│  Settings                                                            │
│  [Organisation]  [Identity (IdP)]  [Admin Users]                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                  [+ Invite Admin]   │
│                                                                      │
│  NAME           EMAIL              ROLE      JOINED                 │
│  ─────────────────────────────────────────────────────────────────  │
│  James Kofi     james@acme.com     Owner     May 10, 2026           │
│  Sarah Mensah   sarah@acme.com     Admin     May 12, 2026           │
│  Kwame Osei     kwame@acme.com     Auditor   May 15, 2026           │
│                                                                      │
│  [Roles]                                                             │
│  Owner:    Full access. Can invite/remove admins.                    │
│  Admin:    Manage apps, policies, gateways. View all logs.           │
│  Auditor:  Read-only. Logs and audit trail only.                     │
│                                                                      │
│  Each row (Owner only): [Change role]  [Remove]                     │
└─────────────────────────────────────────────────────────────────────┘
```

Invite admin (slide-over):

```
┌────────────────────────────────────────┐
│  Invite Admin                     [×]  │
├────────────────────────────────────────┤
│                                        │
│  Email address                         │
│  [ kwame@acme.com                  ]   │
│                                        │
│  Role                                  │
│  ◉ Admin                               │
│  ○ Auditor                             │
│                                        │
│  They will receive an email invite.    │
│  Invite expires in 24 hours.           │
│  They must sign in with acme.com SSO.  │
│                                        │
│  [Cancel]           [Send Invite]      │
└────────────────────────────────────────┘
```

---

## Surface 2: Access Portal
## (access.company.com — server-rendered HTML by gateway binary)

This is the end-user surface. Not the admin dashboard.
Served directly by the Go gateway binary.
Plain HTML + minimal CSS. No JavaScript framework.
Fast. Works on any browser, including mobile.
Server-rendered on every request.

---

### Portal — Authenticated User View

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│  ▪ Acme Corp                               alice@acme.com  [Logout] │
│                                                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Your Applications                                                   │
│                                                                      │
│  ┌──────────────────────┐  ┌──────────────────────┐                 │
│  │  CRM                 │  │  Grafana             │                 │
│  │  crm.acme.com        │  │  grafana.acme.com    │                 │
│  │                      │  │                      │                 │
│  │  ✓ Available         │  │  ✓ Available         │                 │
│  │                      │  │                      │                 │
│  │       [Open →]       │  │       [Open →]       │                 │
│  └──────────────────────┘  └──────────────────────┘                 │
│                                                                      │
│  ┌──────────────────────┐  ┌──────────────────────┐                 │
│  │  GitLab              │  │  HR Portal           │                 │
│  │  git.acme.com        │  │  hr.acme.com         │                 │
│  │                      │  │                      │                 │
│  │  ✓ Available         │  │  ✓ Available         │                 │
│  │                      │  │                      │                 │
│  │       [Open →]       │  │       [Open →]       │                 │
│  └──────────────────────┘  └──────────────────────┘                 │
│                                                                      │
│  ┌──────────────────────┐                                            │
│  │  Wiki                │                                            │
│  │  wiki.acme.com       │                                            │
│  │                      │                                            │
│  │  ✓ Available         │                                            │
│  │                      │                                            │
│  │       [Open →]       │                                            │
│  └──────────────────────┘                                            │
│                                                                      │
│  Apps you do not have access to are not shown.                       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

### Portal — Offline App Card

```
│  ┌──────────────────────┐
│  │  ERP                 │
│  │  erp.acme.com        │
│  │                      │
│  │  ✗ Unavailable       │
│  │    Contact IT        │
│  │                      │
│  │  [Unavailable]       │  ← button is disabled (greyed out)
│  └──────────────────────┘
```

No error details. No internal IP. No stack trace.
Just: unavailable, contact IT.

---

### Portal — Unauthenticated (Login Page)

Shown when user hits any protected app or the portal URL
without an active session.

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│                         ▪  Ashrix Access                            │
│                                                                      │
│                  Sign in to access Acme Corp                        │
│                                                                      │
│              ┌──────────────────────────────────────┐               │
│              │                                      │               │
│              │   [Sign in with Google]              │               │
│              │                                      │               │
│              └──────────────────────────────────────┘               │
│                                                                      │
│              You will be redirected to your company                  │
│              Google account to sign in.                              │
│              No separate password required.                          │
│                                                                      │
│                      Secured by Ashrix Access                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

If the user was trying to reach a specific app:
- After login they are redirected to that app, not the portal
- The login page shows: "Signing in to access: CRM"

---

### Portal — Access Denied Page

Shown when user is authenticated but has no policy for the app.

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│                         ▪  Ashrix Access                            │
│                                                                      │
│                          Access Denied                              │
│                                                                      │
│              You do not have access to this application.            │
│                                                                      │
│              Application:   ERP                                     │
│              Signed in as:  carol@acme.com                          │
│                                                                      │
│              If you believe you should have access,                  │
│              contact your IT administrator.                          │
│                                                                      │
│              [← Back to your applications]                          │
│                                                                      │
│                      Secured by Ashrix Access                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

What this page does NOT show:
- Why access was denied (no policy vs wrong group vs IP block)
- Which groups the user is in
- Which groups would grant access
- Any internal configuration

The user knows two things: denied, contact IT.
No information useful to an attacker.

---

### Portal — Application Unavailable Page

Shown when connector is offline.

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│                         ▪  Ashrix Access                            │
│                                                                      │
│                    Application Unavailable                          │
│                                                                      │
│              ERP is currently unavailable.                          │
│                                                                      │
│              Please try again later or contact                       │
│              your IT administrator.                                  │
│                                                                      │
│              [← Back to your applications]                          │
│                                                                      │
│                      Secured by Ashrix Access                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Surface 3: Error States & Edge Cases

---

### Gateway Offline (User Tries to Reach App)

When a gateway itself is unreachable, the user gets
a standard browser error (connection refused / timeout).
Ashrix cannot intercept this — the gateway IS the front door.
This is why gateway HA (two gateways) is a Phase 1 option.

Admin sees: gateway goes red in dashboard within 30s.
Alert email sent.

---

### Session Expired Mid-Use

User is actively using an app. Session expires.
Gateway silently attempts to re-authenticate with IdP.
If IdP session still valid: new session issued, user continues.
Zero interruption.
If IdP session also expired: user redirected to login page.
After login: returned to the exact page they were on.

---

### MFA Required Page (Step-Up)

Shown when policy has require_mfa: true
and the user's current session does not have MFA claim.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ▪  Ashrix Access                            │
│                                                                      │
│              Additional verification required                       │
│                                                                      │
│              Accessing Finance App requires                          │
│              multi-factor authentication.                            │
│                                                                      │
│              [Verify with Google →]                                 │
│                                                                      │
│              You will complete MFA with your Google account.        │
│              You will be returned here afterwards.                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## UI Build Order (Aligned to Roadmap)

```
Month 4, Week 1-2 (Phase 3):
  Dashboard:
  ✓ Overview page
  ✓ Applications page (list + expand + add)
  ✓ Policies page (list + create rule form)
  ✓ Gateways page (list + enroll flow)
  ✓ Settings: Organisation + IdP + Admin Users

Month 4, Week 3-4 (Phase 3):
  Dashboard:
  ✓ Connectors page (list + generate token)
  ✓ Users page (list + user detail)
  ✓ Sessions page (list + revoke)
  ✓ Access Log page
  ✓ Audit Log page
  ✓ SSE live updates (gateway status, recent events, alerts)

Month 3 (Phase 2):
  Access Portal (served by gateway binary):
  ✓ App list (authenticated)
  ✓ Login page
  ✓ Access denied page
  ✓ App unavailable page
  ✓ MFA step-up page
```

The access portal ships before the dashboard.
Users need to reach their apps.
Admins can use the API (curl) until the dashboard is ready.

---

## The One-Line Summary

```
The access portal: users see only their apps, one click to open.
The admin dashboard: IT sees everything, acts in under 30 seconds.
No VPN instructions anywhere on either surface.
```
