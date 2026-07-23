-- +goose Up
-- -- =================================================================
-- ASHRIX — Initial Schema
-- =================================================================
-- Naming conventions:
--   - All PKs: UUID, gen_random_uuid()
--   - Soft deletes: deleted_at TIMESTAMPTZ (nullable)
--   - Revocations: revoked_at TIMESTAMPTZ (nullable)
--   - All timestamps: TIMESTAMPTZ (timezone-aware)
--   - Private keys: NEVER stored in DB
--   - Secrets (client_secret, token_hash): encrypted/hashed before insert
-- =================================================================


-- -----------------------------------------------------------------
-- ORGS
-- The top-level tenant. Every other entity belongs to an org.
-- slug is immutable after creation — it becomes the subdomain base.
--   e.g. slug=acme → crm.acme.ashrix.io  (default, v1)
--        custom_domain=acme.com → crm.acme.com (v2, requires DNS verify)
-- -----------------------------------------------------------------
CREATE TABLE orgs (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name                        TEXT        NOT NULL,
    slug                        TEXT        NOT NULL UNIQUE,   -- globally unique, immutable  -- e.g. acme → acme.ashrix.io
    custom_domain               TEXT        UNIQUE,            -- e.g. acme.com (v2 feature)
    domain_verified             BOOLEAN     NOT NULL DEFAULT false,
    domain_verification_token   TEXT        UNIQUE,            -- DNS TXT challenge token
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                  TIMESTAMPTZ
);

-- -----------------------------------------------------------------
-- IDP CONFIGS
-- Many per org. Each org can have multiple identity providers.
-- Examples: Google Workspace for staff, Okta for contractors.
-- client_secret MUST be encrypted (AES-256-GCM) before insert.
-- is_verified: set true only after owner completes the SSO binding
--              confirmation flow (one test OIDC login).
-- -----------------------------------------------------------------
CREATE TABLE idp_configs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES orgs(id),
    name            TEXT        NOT NULL,        -- human label: "Google Workspace"
    provider_type   TEXT        NOT NULL CHECK (provider_type IN ('google', 'okta', 'entra', 'oidc','keycloak', 'generic' 'saml')),
    client_id       TEXT        NOT NULL,
    client_secret   TEXT        NOT NULL,        -- AES-256-GCM encrypted, never plaintext
    issuer_url      TEXT        NOT NULL,        -- OIDC discovery base URL
    scopes          TEXT[]      NOT NULL DEFAULT '{}',
    email_claim     TEXT        NOT NULL,
    name_claim      TEXT        NOT NULL,
    group_claim     TEXT        NOT NULL,
    extra_config    JSONB       NOT NULL DEFAULT '{}',
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    is_verified     BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    UNIQUE (org_id, name)
);

-- -----------------------------------------------------------------
-- ADMINS
-- People who manage Ashrix for an org via the dashboard.
-- Two auth phases:
--   Phase 1 (bootstrap): email + password (password_hash NOT NULL)
--   Phase 2 (post-IdP):  OIDC only       (password_hash NULL, sso_only=true)
-- idp_subject stores the `sub` claim from the IdP — permanent binding key.
-- -----------------------------------------------------------------
CREATE TABLE admins (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        REFERENCES orgs(id),
    email           TEXT        NOT NULL UNIQUE,
    password_hash   TEXT,                        -- bcrypt cost>=12; NULL after SSO migration
    idp_subject     TEXT,                        -- IdP `sub` claim; set during SSO binding
    idp_config_id   UUID REFERENCES idp_configs(id),                  
    sso_bound       BOOLEAN     NOT NULL DEFAULT false,
    sso_only        BOOLEAN     NOT NULL DEFAULT false,
    otp_secret TEXT,
    otp_enabled BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT false,
    role            TEXT        NOT NULL CHECK (role IN ('owner', 'admin', 'auditor')),
    invited_by      UUID        REFERENCES  admins(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ,

    UNIQUE (org_id, email),
    UNIQUE (org_id, idp_subject)             -- one admin per IdP subject per org
);


-- -----------------------------------------------------------------
-- ADMIN SESSIONS
-- Dashboard login sessions only.
-- Has nothing to do with end-user (app access) sessions.
-- token_hash: SHA-256 of the actual token. Never store plaintext.
-- -----------------------------------------------------------------
CREATE TABLE admin_sessions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id      UUID        NOT NULL REFERENCES admins(id),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    refresh_token TEXT        NOT NULL UNIQUE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);

-- =================================================================
--Single-use token issued after /register, consumed by /setup-otp + /verify-otp
CREATE TABLE admin_setup_tokens (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE TABLE password_reset_tokens (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);


 -- -----------------------------------------------------------------
-- GATEWAYS
-- Customer-deployed Go binary. One per network location.
-- token_hash: SHA-256 of enrollment token. Used for CP auth.
-- Private key for mTLS lives on the Gateway — never in DB.
-- -----------------------------------------------------------------
CREATE TABLE gateways (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES orgs(id),
    name            TEXT        NOT NULL,
    token_hash      TEXT        NOT NULL UNIQUE, -- SHA-256, never plaintext
    version         TEXT,                        -- reported by gateway on heartbeat
    type            TEXT NOT NULL DEFAULT 'ashrix_hosted' CHECK (type IN ('ashrix_hosted', 'self_hosted')),
    public_url      TEXT NOT NULL, --"gw1.company.com; gateway own public url"
    ip_address      TEXT NOT NULL,
    last_heartbeat  TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'healthy', 'degraded', 'offline')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    enrolled_at TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    
    UNIQUE (org_id, name)
);


-- -----------------------------------------------------------------
-- CONNECTORS
-- Deployed alongside the internal app. Outbound-only tunnel.
-- Belongs to a gateway — bridges the gap between gateway and app.
-- token_hash: used for connector → gateway auth.
-- -----------------------------------------------------------------
CREATE TABLE connectors (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    gateway_id    UUID        NOT NULL REFERENCES gateways(id),
    name          TEXT        NOT NULL,
    token_hash    TEXT        NOT NULL UNIQUE,
    last_seen     TIMESTAMPTZ,
    status        TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'disconnected')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    enrolled_at TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,

    UNIQUE (org_id, name)
);

-- -----------------------------------------------------------------
-- APPS
-- Internal applications published through Ashrix.
-- subdomain: the slug for this app.
--   crm → crm.{org.slug}.ashrix.io
-- upstream: where the connector forwards traffic.
--   e.g. 10.0.1.10:8080
-- Default policy is DENY ALL — no access until a policy rule added.
-- -----------------------------------------------------------------
CREATE TABLE apps (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    connector_id  UUID        REFERENCES connectors(id),
    name          TEXT        NOT NULL,
    subdomain     TEXT        NOT NULL,           -- must be URL-safe slug
    upstream      TEXT        NOT NULL,           -- host:port
    protocol      TEXT        NOT NULL DEFAULT 'http' CHECK (protocol IN ('http', 'tcp', 'ssh')),
    is_public     BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    UNIQUE (org_id, subdomain)
);


--------------------------------------------------------------------
-- Orgs Can Have Multiple IDPS For APPS
--------------------------------------------------------------------
-- Add IdP routing table for apps
CREATE TABLE app_idp_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    idp_id UUID NOT NULL REFERENCES idp_configs(id) ON DELETE CASCADE,
    is_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(app_id, idp_id)
);

-- -----------------------------------------------------------------
-- POLICIES
-- Many per app. Evaluated in priority order (lowest number first).
-- First matching policy wins — OR logic between policies.
-- Within a policy, all rules must match — AND logic between rules.
-- -----------------------------------------------------------------
CREATE TABLE policies (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        UUID        NOT NULL REFERENCES apps(id),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    name          TEXT        NOT NULL,
    priority      INTEGER     NOT NULL DEFAULT 10,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    UNIQUE (app_id, priority)                    -- no two policies share same priority on same app
);


-- -----------------------------------------------------------------
-- POLICY RULES
-- Flat rules. AND logic implicit within a policy.
-- effect: allow | deny
-- rule_type + value pairs:
--   group  + "engineering"
--   email  + "james@company.com"
--   ip     + "196.10.0.0/16"
-- -----------------------------------------------------------------
CREATE TABLE policy_rules (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id     UUID        NOT NULL REFERENCES policies(id),
    effect        TEXT        NOT NULL CHECK (effect IN ('allow', 'deny')),
    rule_type     TEXT        NOT NULL CHECK (rule_type IN ('group', 'email', 'ip')),
    value         TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- -----------------------------------------------------------------
-- REVOCATIONS
-- CP writes revocation events. Gateway polls and consumes.
-- Keeps CP out of the traffic path — Gateway checks this list locally.
-- expires_at: set to now() + max JWT lifetime (e.g. 15 min).
--   Gateway can safely purge expired entries.
-- target_id: session token hash, gateway id, or connector id.
-- -----------------------------------------------------------------
CREATE TABLE revocations (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    type          TEXT        NOT NULL CHECK (type IN ('session', 'connector', 'gateway')),
    target_id     TEXT        NOT NULL,
    reason        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);

-- -----------------------------------------------------------------
-- CA CERTIFICATES
-- Root CA and Intermediate CA records.
-- Public cert stored. Private key NEVER in DB — lives in secret manager.
-- -----------------------------------------------------------------
CREATE TABLE ca_certificates (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL UNIQUE, -- "Ashrix Root CA"
    type            TEXT        NOT NULL CHECK (type IN ('root', 'intermediate')),
    cert_pem        TEXT        NOT NULL,        -- public cert only
    serial_number   TEXT        NOT NULL UNIQUE,
    subject         TEXT        NOT NULL,
    issued_at       TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
    -- private key: secret manager / HSM only
);


-- -----------------------------------------------------------------
-- COMPONENT CERTIFICATES
-- One cert per gateway or connector.
-- Private key generated on the component via CSR flow — never stored here.
-- rotation_of: links new cert back to the cert it replaced.
-- -----------------------------------------------------------------
CREATE TABLE component_certificates (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID        NOT NULL REFERENCES orgs(id),
    component_type      TEXT        NOT NULL CHECK (component_type IN ('gateway', 'connector')),
    component_id        UUID        NOT NULL,    -- gateway_id or connector_id
    ca_id               UUID        NOT NULL REFERENCES ca_certificates(id),
    cert_pem            TEXT        NOT NULL,    -- public cert only
    serial_number       TEXT        NOT NULL UNIQUE,
    subject             TEXT        NOT NULL,    -- CN=gateway-london-hq
    san                 TEXT[],                  -- Subject Alternative Names
    issued_at           TIMESTAMPTZ NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    revoke_reason       TEXT CHECK (revoke_reason IN ('keyCompromise', 'superseded', 'cessationOfOperation', 'affiliationChanged')),
    rotation_of         UUID        REFERENCES component_certificates(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
    -- private key lives on the component that generated the CSR
);

-- -----------------------------------------------------------------
-- CSR REQUESTS
-- Gateway/connector submits a CSR. CP signs it and returns the cert.
-- CSR submission gated by valid enrollment token — never unauthenticated.
-- -----------------------------------------------------------------
CREATE TABLE csr_requests (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES orgs(id),
    component_type  TEXT        NOT NULL CHECK (component_type IN ('gateway', 'connector')),
    component_id    UUID        NOT NULL,
    csr_pem         TEXT        NOT NULL,        -- the raw CSR from the component
    status          TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'signed', 'rejected')),
    signed_cert_id  UUID        REFERENCES component_certificates(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ
);


-- -----------------------------------------------------------------
-- CRL ENTRIES (Certificate Revocation List)
-- Append-only. Gateway fetches this list and rejects revoked certs.
-- -----------------------------------------------------------------
CREATE TABLE crl_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cert_id         UUID        NOT NULL REFERENCES component_certificates(id),
    serial_number   TEXT        NOT NULL UNIQUE,
    revoked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason          TEXT        NOT NULL CHECK (reason IN ('keyCompromise', 'superseded', 'cessationOfOperation', 'affiliationChanged'))
);


-- -----------------------------------------------------------------
-- AUDIT LOGS
-- Append-only. Records every admin action.
-- actor_id NULL = system-generated event (e.g. cert auto-rotated).
-- details JSONB: flexible per action type — no fixed schema.
-- -----------------------------------------------------------------
CREATE TABLE audit_logs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    actor_id      UUID        REFERENCES admins(id),  -- NULL = system action
    action        TEXT        NOT NULL,               -- app.created | policy.deleted
    target_type   TEXT,                               -- app | policy | gateway | connector
    target_id     TEXT,
    details       JSONB,                              -- action-specific metadata
    ip            TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- -----------------------------------------------------------------
-- ACCESS LOGS
-- Written async by Gateway. CP never in the traffic path.
-- idp_config_id: which IdP authenticated this user.
-- deny_reason NULL when result=allowed.
-- -----------------------------------------------------------------
CREATE TABLE access_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES orgs(id),
    app_id          UUID        REFERENCES apps(id),
    idp_config_id   UUID        REFERENCES idp_configs(id),
    user_email      TEXT,
    method          TEXT,
    path            TEXT,
    status          INTEGER,
    latency_ms      INTEGER,
    ip              TEXT,
    result          TEXT        NOT NULL CHECK (result IN ('allowed', 'denied')),
    deny_reason     TEXT CHECK (deny_reason IN ('no_session', 'policy_deny', 'ip_deny', 'session_revoked', 'app_offline')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- =================================================================
-- INDEXES
-- =================================================================

-- Admin dashboard login (hot path for dashboard)
CREATE INDEX idx_admins_org_email
    ON admins (org_id, email);

CREATE INDEX idx_admin_sessions_token
    ON admin_sessions (refresh_token)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_admin_sessions_admin
    ON admin_sessions (admin_id)
    WHERE revoked_at IS NULL;

-- Gateway heartbeat + CP auth (to use mTLS, so this may be unnecessary)
CREATE INDEX idx_gateways_token
    ON gateways (token_hash)
     WHERE revoked_at IS NULL;
    
CREATE INDEX idx_gateways_org
    ON gateways (org_id)
    WHERE revoked_at IS NULL;

-- Connector auth
CREATE INDEX idx_connectors_token
    ON connectors (token_hash)
    WHERE revoked_at IS NULL;
    
 CREATE INDEX idx_connectors_gateway
    ON connectors (gateway_id)
     WHERE revoked_at IS NULL;

-- App lookup by subdomain (policy sync + routing)
CREATE INDEX idx_apps_org_subdomain
    ON apps (org_id, subdomain)
    WHERE deleted_at IS NULL;

-- Policy sync: Gateway pulls policies for its org
CREATE INDEX idx_policies_app
    ON policies (app_id)
    WHERE deleted_at IS NULL AND is_active = true;

CREATE INDEX idx_policy_rules_policy
    ON policy_rules (policy_id);

-- -- Revocation polling by Gateway
-- CREATE INDEX idx_revocations_org_time
--     ON revocations (org_id, created_at DESC)
--     WHERE expires_at > now();

-- Certificate expiry monitoring (alert before expiry)
CREATE INDEX idx_component_certs_expiry
    ON component_certificates (expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_component_certs_component
    ON component_certificates (component_type, component_id)
    WHERE revoked_at IS NULL;

-- Pending CSR queue
CREATE INDEX idx_csr_pending
    ON csr_requests (status)
    WHERE status = 'pending';

-- CRL lookup by serial (Gateway cert validation)
CREATE INDEX idx_crl_serial
    ON crl_entries (serial_number);

-- Dashboard queries: access logs
CREATE INDEX idx_access_logs_org_time
    ON access_logs (org_id, created_at DESC);

CREATE INDEX idx_access_logs_app_time
    ON access_logs (app_id, created_at DESC);

CREATE INDEX idx_access_logs_denied
    ON access_logs (org_id, created_at DESC)
    WHERE result = 'denied';


-- Dashboard queries: audit logs
CREATE INDEX idx_audit_logs_org_time
    ON audit_logs (org_id, created_at DESC);

CREATE INDEX idx_audit_logs_actor
    ON audit_logs (actor_id, created_at DESC);

-- IDP config lookup
CREATE INDEX idx_idp_configs_org
    ON idp_configs (org_id)
    WHERE deleted_at IS NULL AND is_active = true;

-- Custom domain lookup (Gateway resolves incoming hostname to org)
CREATE INDEX idx_orgs_custom_domain
    ON orgs (custom_domain)
    WHERE custom_domain IS NOT NULL
      AND domain_verified = true
      AND deleted_at IS NULL;


-- +goose Down
DROP INDEX IF EXISTS idx_orgs_custom_domain;
DROP INDEX IF EXISTS idx_idp_configs_org;
DROP INDEX IF EXISTS idx_audit_logs_actor;
DROP INDEX IF EXISTS idx_audit_logs_org_time;
DROP INDEX IF EXISTS idx_access_logs_denied;
DROP INDEX IF EXISTS idx_access_logs_app_time;
DROP INDEX IF EXISTS idx_access_logs_org_time;
DROP INDEX IF EXISTS idx_crl_serial;
DROP INDEX IF EXISTS idx_csr_pending;
DROP INDEX IF EXISTS idx_component_certs_component;
DROP INDEX IF EXISTS idx_component_certs_expiry;
DROP INDEX IF EXISTS idx_revocations_org_time;
DROP INDEX IF EXISTS idx_policy_rules_policy;
DROP INDEX IF EXISTS idx_policies_app;
DROP INDEX IF EXISTS idx_apps_org_subdomain;
DROP INDEX IF EXISTS idx_connectors_gateway;
DROP INDEX IF EXISTS idx_connectors_token;
DROP INDEX IF EXISTS idx_gateways_org;
DROP INDEX IF EXISTS idx_gateways_token;
DROP INDEX IF EXISTS idx_admin_sessions_admin;
DROP INDEX IF EXISTS idx_admin_sessions_token;


DROP TABLE IF EXISTS access_logs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS crl_entries;
DROP TABLE IF EXISTS csr_requests;
DROP TABLE IF EXISTS component_certificates;
DROP TABLE IF EXISTS ca_certificates;
DROP TABLE IF EXISTS revocations;
DROP TABLE IF EXISTS policy_rules;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS connectors;
DROP TABLE IF EXISTS gateways;


DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS idp_configs;
DROP TABLE IF EXISTS orgs;
