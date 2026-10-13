# Authentication

Authentication establishes the identity associated with a user or component. It is separate from the gateway's authorization decision and enforcement of access to an application.

## User Authentication Flow

The documented browser flow uses a tenant identity provider through OIDC:

1. An unauthenticated browser request reaches the gateway.
2. The gateway initiates the sign-in flow and preserves a short-lived state value to bind the callback to the original request.
3. The Control Plane redirects the user to the configured identity provider.
4. The identity provider authenticates the user and returns an authorization response to the Control Plane.
5. The Control Plane validates the OIDC response and issues a short-lived, single-use ticket for the gateway.
6. The gateway redeems the ticket, evaluates application policy, and creates its local session only if access is allowed.

See the [request lifecycle](../02-architecture/request-lifecycle.md) for the documented sequence. OIDC provider configuration, allowed redirect URIs, state validation, nonce handling, and token validation are security-critical and should be checked against the current implementation.

## Browser Session

The gateway uses a browser cookie to associate later requests with the local session. The design documents a host-scoped, secure, HttpOnly cookie and server-side session state. Cookie flags reduce specific risks but do not make a stolen session harmless; expiration, revocation, and policy re-evaluation remain important.

## First Admin Login and TOTP Enrollment

The initial organization and owner are created with `ashrix-cp setup bootstrap`; the CLI does not create a TOTP setup token. The admin is active with TOTP disabled and can use the bootstrap password only to receive a short-lived enrollment challenge.

The dashboard uses that challenge to request a TOTP secret and QR code, then submits the first six-digit code for verification. The challenge is purpose-scoped: it can access the first-login TOTP setup and verification endpoints, not normal OTP login or protected dashboard APIs. TOTP is enabled only after successful verification, at which point the CP issues the normal access/refresh session. Password login for an account without TOTP configured follows this enrollment path.

If the challenge expires before enrollment completes, the admin must sign in again with the password. Retries reuse the pending secret until it is successfully verified.

## Component Authentication

Gateway, connector, and Control Plane authenticate one another with mTLS identity certificates on their management connections. Connector and gateway bootstrap enrollment is a separate machine-identity process, not user sign-in. See [mTLS](./mtls.md) for certificate details.

## Authorization Is a Separate Check

After authentication, the gateway must still evaluate whether the user may access the requested application. A failed or unavailable authentication flow must not create an authorized session. A successful OIDC response must not be treated as blanket permission.

## Status and Verification

The first-login flow uses purpose-scoped JWT challenges and a conditional database update to enable TOTP once. Password rate limiting, TOTP-secret encryption at rest, challenge expiry, and session revocation still require verification before treating the full admin-authentication lifecycle as production-hardened.

---

*Next: [mTLS and Certificate Lifecycle](./mtls.md)*