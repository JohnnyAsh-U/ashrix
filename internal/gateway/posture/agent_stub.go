package posture

import "time"

// ============================================================
// 11. POST-MVP: AGENT-BASED POSTURE (stub)
// ============================================================
// When you deploy an endpoint agent, the posture struct expands.
// The agent reports via a separate authenticated channel.
// ============================================================

// AgentPostureReport is sent by the endpoint agent to the CP.
type AgentPostureReport struct {
	DeviceID        string `json:"device_id"`
	TenantID        string `json:"tenant_id"`
	UserID          string `json:"user_id"`

	// OS-level signals (impossible to get from browser)
	DiskEncrypted   bool   `json:"disk_encrypted"`
	EncryptionType  string `json:"encryption_type"` // "filevault", "bitlocker", "luks"
	AVRunning       bool   `json:"av_running"`
	AVProduct       string `json:"av_product"`      // "crowdstrike", "sentinelone"
	OSPatchLevel    string `json:"os_patch_level"`  // "2024-001"
	ScreenLockEnabled bool `json:"screen_lock_enabled"`
	PasswordPolicy  string `json:"password_policy"` // "strong", "weak"
	FirewallEnabled bool   `json:"firewall_enabled"`

	ReportedAt      time.Time `json:"reported_at"`
}

// The CP stores agent reports and the Gateway queries them
// when building the AuthorizationContext:
//
//   agentReport, _ := agentStore.GetLatestReport(userID, deviceID)
//   posture.Status = determineCompliance(agentReport)
//
// For MVP, agentReport is nil and Status is always "passive".
