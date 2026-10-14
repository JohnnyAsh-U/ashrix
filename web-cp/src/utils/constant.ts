export const LoginUrl = '/auth/login'
export const LogoutUrl = '/auth/logout'
export const RefreshTokenUrl = '/auth/refresh'
export const MeUrl = '/auth/me'
export const Verify2FALoginUrl = '/auth/verify-otp/login'
export const FirstLoginTOTPSetupUrl = '/auth/first-login/totp/setup'
export const FirstLoginTOTPVerifyUrl = '/auth/first-login/totp/verify'
export const Setup2FAUrl = '/auth/2fa/setup'
export const Enable2FAUrl = '/auth/2fa/enable'
export const Disable2FAUrl = '/auth/2fa/disable'
export const ChangePasswordUrl = '/auth/change-password'

// Gateway endpoints
export const GatewaysUrl = '/gateways'
export const GatewayUrl = (id: string) => `${GatewaysUrl}/${id}`
export const RecreateGatewayUrl = (id: string) => `${GatewayUrl(id)}/re-create`
export const RevokeGatewayUrl = (id: string) => `${GatewayUrl(id)}/revoke`
export const RotateGatewayCertificateUrl = (id: string) => `${GatewayUrl(id)}/certs/rotate`
export const DrainGatewayUrl = (id: string) => `${GatewayUrl(id)}/drain`

// Connector endpoints
export const ConnectorsUrl = '/connectors'
export const ConnectorUrl = (id: string) => `${ConnectorsUrl}/${id}`
export const RecreateConnectorUrl = (id: string) => `${ConnectorUrl(id)}/re-create`
export const RevokeConnectorUrl = (id: string) => `${ConnectorUrl(id)}/revoke`
export const RevokeConnectorCertificateUrl = (id: string) => `${ConnectorUrl(id)}/certs/revoke`
export const RotateConnectorCertificateUrl = (id: string) => `${ConnectorUrl(id)}/certs/rotate`
export const RenewConnectorCertificateUrl = '/internal/connectors/renew'

// Application endpoints
export const ApplicationsUrl = '/apps'
export const ApplicationUrl = (id: string) => `${ApplicationsUrl}/${id}`

// Log endpoints
export const AccessLogsUrl = '/logs/access'

// Active user session endpoints
export const ActiveSessionsUrl = '/idp-configs/sessions'
export const RevokeSessionUrl = (id: string) => `/idp-configs/revoke-session/${id}`

// Identity provider endpoints
export const IdentityConfigsUrl = '/idp-configs'

// Policy endpoints
export const PoliciesUrl = '/policies'
export const PolicyUrl = (id: string) => `${PoliciesUrl}/${id}`



