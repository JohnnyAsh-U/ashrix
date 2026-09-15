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
    provider_type   TEXT        NOT NULL CHECK (provider_type IN ('google', 'okta', 'entra', 'oidc','keycloak', 'generic', 'saml')),
    client_id       TEXT        NOT NULL,
    client_secret   TEXT        NOT NULL,        -- AES-256-GCM encrypted, never plaintext
    issuer_url      TEXT        NOT NULL,        -- OIDC discovery base URL
    scopes          TEXT[]        NOT NULL DEFAULT '{}',
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
    deployment_type            TEXT NOT NULL DEFAULT 'hosted' CHECK (deployment_type IN ('hosted', 'self_hosted')),
    public_url      TEXT NOT NULL, --"gw1.company.com; gateway own public url"
    quic_port       TEXT,
    grpc_port       TEXT,
    https_port       TEXT,
    ip_address      TEXT NOT NULL,
    last_heartbeat  TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'healthy', 'degraded', 'offline', 'draining')),
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    log_to_cp  BOOLEAN     NOT NULL DEFAULT true,
    uptime          BIGINT      NOT NULL DEFAULT 0,
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
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID        NOT NULL REFERENCES orgs(id),
    gateway_id           UUID        NOT NULL REFERENCES gateways(id),
    secondary_gateway_id UUID        REFERENCES gateways(id),
    name                 TEXT        NOT NULL,
    token_hash           TEXT        NOT NULL UNIQUE,
    last_seen            TIMESTAMPTZ,
    status               TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'disconnected')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_active            BOOLEAN     NOT NULL DEFAULT true,
    open_sock            BOOLEAN     NOT NULL DEFAULT false,
    active_streams       INT         NOT NULL DEFAULT 0,
    enrolled_at          TIMESTAMPTZ,
    revoked_at           TIMESTAMPTZ,

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
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID        NOT NULL REFERENCES orgs(id),
    connector_id            UUID        REFERENCES connectors(id),
    name                    TEXT        NOT NULL,
    subdomain               TEXT        NOT NULL,           -- must be URL-safe slug
    upstream                TEXT        NOT NULL,           -- host:port
    protocol                TEXT        NOT NULL DEFAULT 'http' CHECK (protocol IN ('http', 'tcp', 'ssh')),
    is_public               BOOLEAN     NOT NULL DEFAULT false,
    enable_security_headers BOOLEAN     NOT NULL DEFAULT true,
    check_health            BOOLEAN     NOT NULL DEFAULT true,
    check_interval          INT         NOT NULL DEFAULT 60,
    health_endpoint         TEXT,           -- health check endpoint (relative path)
    health_status           TEXT        NOT NULL DEFAULT 'unknown' CHECK (health_status IN ('unknown', 'healthy', 'unhealthy')),
    last_seen               TIMESTAMPTZ,

    sock_pass               TEXT        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ,

    UNIQUE (org_id, subdomain)
);

-------------------------------------------------------------------
-- User Sessions
-------------------------------------------------------------------
CREATE TABLE user_sessions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
    user_id       UUID        NOT NULL,
    user_email TEXT NOT NULL DEFAULT '',
    gateway_id    UUID        NOT NULL REFERENCES gateways(id),
    issued_at     TIMESTAMPTZ NOT NULL,
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ NULL
);

CREATE INDEX idx_user_sessions_user
    ON user_sessions(user_id, org_id, revoked_at);


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

-- ============================================================
-- POLICIES
-- DESIGN PRINCIPLE:
--   - Normalize subjects and resources (queryable, enforceable FKs)
--   - Keep conditions as JSONB (evaluated as a unit, deeply nested)
--   - Keep mutation log as denormalized snapshots (distribution)

-- =================================================================

-- -----------------------------------------------------------
-- POLICY_MUTATIONS — Append-only changelog (unchanged)
-- -----------------------------------------------------------
-- The snapshot here is DENORMALIZED — it captures the full
-- policy state (subjects, resources, conditions) as one JSONB
-- document. This is the distribution format, not the query format.
-- -----------------------------------------------------------

CREATE TABLE policy_mutations (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    sequence         BIGINT NOT NULL,
    org_id       UUID NOT NULL REFERENCES orgs(id),
    policy_id       UUID NOT NULL,
    op              VARCHAR(10) NOT NULL CHECK (op IN ('UPSERT', 'DELETE')),

    -- DENORMALIZED snapshot: full policy state at this version.
    -- Built by joining policies + policy_subjects + policy_resources
    -- + policy_conditions at write time. Used for delta computation
    -- and as a historical record.
    rule_snapshot   JSONB NOT NULL,
    old_rule_snapshot     JSONB,
    
    version        BIGINT NOT NULL DEFAULT 1,
    signature       BYTEA NOT NULL,   -- SHA256 digest of the bundle
    record_timestamp BIGINT NOT NULL DEFAULT 0, -- timestamp of when the policy was recorded

    mutated_by      UUID NULL REFERENCES admins(id),
    mutated_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL,

    ip_address      INET,
    user_agent      TEXT
);

CREATE INDEX idx_mutations_tenant_version
    ON policy_mutations(org_id, version);

CREATE INDEX idx_mutations_policy
    ON policy_mutations(policy_id, version);

-- Critical: prevents replay attacks by enforcing monotonic sequence per policy
CREATE UNIQUE INDEX idx_mutations_org_sequence
    ON policy_mutations(org_id, policy_id, sequence);


-- Fast lookup for "give me all mutations for this policy since sequence X"
CREATE INDEX idx_mutations_policy_version 
    ON policy_mutations(policy_id, sequence); 


-- =================================================================
-- Policy Precedence in strict effect order: 
--   - (1) all explicit DENY policies, 
--   - (2) all explicit ALLOW policies, 
--   - (3) default DENY. 
-- Within each effect class, evaluation order is deterministic but not user-configurable in MVP. 
-- A matched DENY cannot be overridden by an ALLOW.
-- ============================================================


CREATE TABLE policies (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    effect          VARCHAR(10) NOT NULL CHECK (effect IN ('ALLOW', 'DENY')),
    priority        INT DEFAULT 0 CHECK (priority >= 0 AND priority <= 100),
    enabled         BOOLEAN DEFAULT TRUE NOT NULL,

    version         BIGINT NOT NULL DEFAULT 1,
    sequence         BIGINT NOT NULL DEFAULT 1, -- From the mutation table, to make queries easier

    created_by      UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,

    created_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_policies_orgs
    ON policies(org_id);

CREATE INDEX idx_policies_org_enabled
    ON policies(org_id, enabled);

CREATE INDEX idx_policies_compile_order
    ON policies(org_id, effect DESC, priority DESC, id);


-- -----------------------------------------------------------
-- POLICY_SUBJECTS — Normalized subject references
-- -----------------------------------------------------------
-- Why normalized? So you can query:
--   "Which policies reference group 'engineering'?"
--   "Delete group X and remove all references."
--   "Rename group X to Y across all policies."
-- -----------------------------------------------------------
CREATE TABLE policy_subjects (
    policy_id       UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    subject_type    VARCHAR(20) NOT NULL CHECK (subject_type IN ('group', 'user', 'app')),
    subject_value   VARCHAR(255) NOT NULL,

    PRIMARY KEY (policy_id, subject_type, subject_value)
);

-- Fast lookup: "which policies use group 'engineering'?"
CREATE INDEX idx_subjects_lookup
    ON policy_subjects(subject_type, subject_value, policy_id);

-- Fast lookup: "all subjects for policy X"
CREATE INDEX idx_subjects_policy
    ON policy_subjects(policy_id);


-- -----------------------------------------------------------
-- POLICY_RESOURCES — Normalized resource references
-- -----------------------------------------------------------
-- Same rationale as subjects. Resources are queried independently.
-- -----------------------------------------------------------

CREATE TABLE policy_resources (
    policy_id       UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    resource_type   VARCHAR(20) NOT NULL CHECK (resource_type IN ('app', 'path', 'method')),
    resource_value  VARCHAR(255) NOT NULL,

    PRIMARY KEY (policy_id, resource_type, resource_value)
);

CREATE INDEX idx_resources_lookup
    ON policy_resources(resource_type, resource_value, policy_id);

CREATE INDEX idx_resources_policy
    ON policy_resources(policy_id);

-- -----------------------------------------------------------
-- SCHEDULES — Reusable time windows
-- -----------------------------------------------------------
-- Policies reference schedules by name instead of embedding
-- time rules. This lets an admin change "business hours" in
-- one place and have 20 policies update automatically.
-- -----------------------------------------------------------

CREATE TABLE schedules (
    id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    timezone        VARCHAR(64) DEFAULT 'UTC' NOT NULL,

    -- Array of daily rules:
    -- [
    --   {"day": "monday", "start": "09:00", "end": "18:00"},
    --   {"day": "tuesday", "start": "09:00", "end": "18:00"}
    -- ]
    rules           JSONB NOT NULL DEFAULT '[]',

    created_by      UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_schedules_tenant
    ON schedules(org_id);


-- -----------------------------------------------------------
-- POLICY_CONDITIONS — JSONB condition tree
-- -----------------------------------------------------------
-- Conditions are NOT normalized because:
--   1. They are evaluated as a single unit (all must match)
--   2. They are deeply nested and evolve (new condition types)
--   3. You never query "which policies have block_tor=true?" independently
--   4. The condition tree is a document, not a relation
-- -----------------------------------------------------------

CREATE TABLE policy_conditions (
    policy_id       UUID PRIMARY KEY REFERENCES policies(id) ON DELETE CASCADE,
    condition_tree  JSONB NOT NULL DEFAULT '{}',
    -- Example:
    -- {
    --   "mfa": { "required": true, "min_level": "totp" },
    --   "device": { "postures": ["compliant"] },
    --   "network": {
    --     "allowed_countries": ["CI", "GH"],
    --     "blocked_countries": [],
    --     "allowed_cidrs": ["102.68.0.0/16"],
    --     "block_tor": true
    --   },
    --   "time": { "schedule_name": "business-hours" }
    -- }

    updated_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

-- GIN index for partial condition queries (if needed later)
CREATE INDEX idx_conditions_gin
    ON policy_conditions USING GIN(condition_tree);


-- ==========================================================================================
-- Gateway_Events
-- =========================================================================================

CREATE TABLE gateway_events (
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    event_id UUID NOT NULL DEFAULT gen_random_uuid(),
    command TEXT NOT NULL,
    delivery_mode TEXT NOT NULL CHECK (delivery_mode IN ('ACTION', 'SNAPSHOT')),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gateway_id, seq),
    UNIQUE (event_id)
);

CREATE TABLE gateway_events_acks (
    gateway_id UUID PRIMARY KEY REFERENCES gateways(id) ON DELETE CASCADE,
    last_acked_seq BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE INDEX idx_gateway_events_gateway_seq
    ON gateway_events(gateway_id, seq);

CREATE TABLE gateway_event_sequences (
    gateway_id UUID PRIMARY KEY REFERENCES gateways(id) ON DELETE CASCADE,
    next_seq BIGINT NOT NULL DEFAULT 1
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
    org_id              UUID        REFERENCES orgs(id),
    component_type      TEXT        NOT NULL CHECK (component_type IN ('cp','gateway', 'connector')),
    component_id        UUID,    -- gateway_id or connector_id or cp
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
-- CRL ENTRIES (Certificate Revocation List)
-- Append-only. Gateway fetches this list and rejects revoked certs.
-- -----------------------------------------------------------------
CREATE TABLE crl_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id),
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
    gateway_id      UUID        REFERENCES gateways(id),
    app_id          UUID        REFERENCES apps(id),
    policy_id       UUID        REFERENCES policy_mutations(id),
    user_id         TEXT,
    user_email      TEXT,
    method          TEXT,
    path            TEXT,
    status          INTEGER,
    latency_ms      INTEGER,
    action          TEXT,
    ip              TEXT,
    result          TEXT        NOT NULL CHECK (result IN ('allowed', 'denied')),
    deny_reason     TEXT CHECK (deny_reason IN ('no_session', 'policy_deny', 'ip_deny', 'session_revoked', 'app_offline')),
    bytes_in        BIGINT NOT NULL DEFAULT 0,
    bytes_out       BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- =================================================================
-- INDEXES
-- =================================================================

CREATE INDEX idx_user_sessions_org_email_active
ON user_sessions(org_id, user_email, revoked_at, expires_at);


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
-- CREATE INDEX idx_csr_pending
--     ON csr_requests (status)
--     WHERE status = 'pending';

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

DROP INDEX IF EXISTS idx_mutations_tenant_version;
DROP INDEX IF EXISTS idx_mutations_policy;
DROP INDEX IF EXISTS idx_policies_orgs;
DROP INDEX IF EXISTS idx_policies_org_enabled;
DROP INDEX IF EXISTS idx_policies_compile_order;
DROP INDEX IF EXISTS idx_subjects_lookup;
DROP INDEX IF EXISTS idx_subjects_policy;

DROP INDEX IF EXISTS idx_schedules_tenant;


DROP INDEX IF EXISTS idx_apps_org_subdomain;
DROP INDEX IF EXISTS idx_connectors_gateway;
DROP INDEX IF EXISTS idx_connectors_token;
DROP INDEX IF EXISTS idx_gateways_org;
DROP INDEX IF EXISTS idx_gateways_token;
DROP INDEX IF EXISTS idx_admin_sessions_admin;
DROP INDEX IF EXISTS idx_resources_lookup;
DROP INDEX IF EXISTS idx_resources_policy;
DROP INDEX IF EXISTS idx_admin_sessions_token;
DROP INDEX IF EXISTS idx_conditions_gin;

DROP INDEX IF EXISTS idx_mutations_policy_version;
DROP INDEX IF EXISTS idx_mutations_policy_sequence;
DROP INDEX IF EXISTS idx_gateway_events_gateway_seq;


DROP TABLE IF EXISTS access_logs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS crl_entries; 
-- DROP TABLE IF EXISTS csr_requests;
DROP TABLE IF EXISTS component_certificates;
DROP TABLE IF EXISTS ca_certificates;
DROP TABLE IF EXISTS revocations;



DROP TABLE IF EXISTS gateway_policy_acks;
DROP TABLE IF EXISTS policy_versions;
DROP TABLE IF EXISTS policy_subjects;
DROP TABLE IF EXISTS policy_resources;
DROP TABLE IF EXISTS policy_conditions;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS policy_mutations;
DROP TABLE IF EXISTS schedules;




DROP TABLE IF EXISTS gateway_events_acks;
DROP TABLE IF EXISTS gateway_event_sequences;
DROP TABLE IF EXISTS gateway_events;

DROP TABLE IF EXISTS app_idp_mappings;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS connectors;
DROP TABLE IF EXISTS gateways;

DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_setup_tokens;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS idp_configs;
DROP TABLE IF EXISTS orgs;


-- -----------------------------------------------------------
-- 7. ATOMIC CREATION TRANSACTION (revised)
-- -----------------------------------------------------------
-- Now inserts into 5 tables atomically:
--   1. policy_mutations (gets version)
--   2. policies (core metadata)
--   3. policy_subjects (normalized subjects)
--   4. policy_resources (normalized resources)
--   5. policy_conditions (JSONB tree)
--   6. policy_audit_log
-- -----------------------------------------------------------
-- End of migration

