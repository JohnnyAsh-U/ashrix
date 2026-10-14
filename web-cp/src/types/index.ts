/**
 * Ecclesix Control Plane - Type Definitions
 * Core types for multi-tenant SaaS admin dashboard
 */

// ============================================================================
// TENANT TYPES
// ============================================================================

export type TenantStatus = 'active' | 'suspended' | 'trial' | 'archived'
export type TenantPlan = 'Starter' | 'Professional' | 'Enterprise' | 'Custom' | string

export interface TenantStorage {
  quota_bytes: number
  used_bytes: number
  percent: number
}

export interface Tenant {
  id: number
  church_name: string
  domains: string[]
  plan: TenantPlan | null
  is_active: boolean
  church_count: number
  member_count: number
  storage?: TenantStorage
  name?: string
  email?: string
  domain?: string
  custom_logo: boolean
  phone:string
  subdomain?: string
  status?: TenantStatus
  usersCount?: number
  storageUsedGB?: number
  storageQuotaGB?: number
  lastLoginAt?: string | null
  createdAt?: string
  updatedAt?: string
  suspendedAt?: string | null
  suspendedReason?: string | null
}

export interface TenantDetail extends Tenant {
  adminUser: {
    id: string
    name: string
    email: string
  }
  billingEmail: string
  supportEmail: string
  organization: string
  website: string | null
  databaseSchema: string
  backupRetentionDays: number
  maxApiCalls: number
  customDomain: string | null
}

export interface TenantMetrics {
  tenantId: string
  activeUsers: number
  totalRequests: number
  successRate: number
  errorRate: number
  avgResponseTime: number
  lastUpdated: string
}



// -----------------------------------------------------------
// Billing Plan

export interface BillingPlan {
  id: number,
  code: string,
  name: string,
  price: string,
  annual_price: string,
  currency: string,
  max_churches: number,
  max_members: number,
  created_at : string,
  updated_at: string
}

// ============================================================================
// BILLING TYPES
// ============================================================================

export type BillingStatus = 'paid' | 'pending' | 'failed' | 'overdue'
export type PaymentMethod = 'credit_card' | 'bank_transfer' | 'manual'

export interface Subscription {
  id: string
  tenantId: string
  tenantName: string
  plan: TenantPlan
  status: 'active' | 'cancelled' | 'suspended'
  currentPeriodStart: string
  currentPeriodEnd: string
  cancelledAt?: string | null
  autoRenew: boolean
  createdAt: string
}

export interface Invoice {
  id: string
  tenantId: string
  tenantName: string
  amount: number
  currency: string
  status: BillingStatus
  dueDate: string
  issuedDate: string
  paidDate?: string | null
  description: string
  itemCount: number
}

export interface BillingMetrics {
  mrr: number // Monthly Recurring Revenue
  arr: number // Annual Recurring Revenue
  churnRate: number
  failedPaymentsCount: number
  failedPaymentsPercentage: number
  activeSubscriptions: number
  activeTrials: number
}

export interface Coupon {
  id: string
  code: string
  description: string
  discountType: 'percentage' | 'fixed'
  discountValue: number
  maxUses: number
  currentUses: number
  isActive: boolean
  expiresAt: string | null
  createdAt: string
}

export interface Plan {
  id: string
  name: TenantPlan
  displayName: string
  description: string
  monthlyPrice: number
  annualPrice: number
  features: string[]
  userLimit: number
  storageGBLimit: number
  apiCallsLimit: number
  isActive: boolean
}

// ============================================================================
// BACKUP & RESTORE TYPES
// ============================================================================

export type BackupStatus = 'pending' | 'in_progress' | 'completed' | 'failed'
export type BackupType = 'manual' | 'scheduled' | 'on_demand'

export interface Backup {
  id: string
  tenantId: string
  tenantName: string
  type: BackupType
  status: BackupStatus
  sizeGB: number
  createdAt: string
  completedAt?: string | null
  expiresAt: string
  retentionDays: number
  failureReason?: string | null
}

export interface RestoreJob {
  id: string
  backupId: string
  tenantId: string
  targetEnvironment: 'production' | 'staging'
  status: 'pending' | 'in_progress' | 'completed' | 'failed'
  startedAt: string
  completedAt?: string | null
  failureReason?: string | null
}

// ============================================================================
// STORAGE TYPES
// ============================================================================

export interface StorageFile {
  id: string
  tenantId: string
  name: string
  path: string
  sizeBytes: number
  type: string
  createdAt: string
  lastModified: string
  isOrphan: boolean
}

export interface StorageMetrics {
  tenantId: string
  totalFilesCount: number
  totalSizeGB: number
  orphanFilesCount: number
  orphanSizeGB: number
  largestFileGB: number
  tempFilesCount: number
  tempSizeGB: number
}

// ============================================================================
// SUPPORT & MAIL CENTER TYPES
// ============================================================================

export type EmailInboxType = 'support' | 'billing' | 'sales'
export type EmailPriority = 'low' | 'medium' | 'high' | 'urgent'
export type EmailStatus = 'open' | 'assigned' | 'pending_response' | 'resolved'

export interface Email {
  id: string
  inboxType: EmailInboxType
  subject: string
  from: string
  to: string
  body: string
  priority: EmailPriority
  status: EmailStatus
  assignedTo?: string | null
  tags: string[]
  slaDeadline?: string | null
  slaBreached: boolean
  receivedAt: string
  responseCount: number
  lastResponseAt?: string | null
}

export interface SupportTicket {
  id: string
  emailId: string
  tenantId?: string | null
  tenantName?: string | null
  status: EmailStatus
  category: string
  assignedTo?: string | null
  priority: EmailPriority
  createdAt: string
  resolvedAt?: string | null
  totalTime: number // minutes
}

// ============================================================================
// METRICS & MONITORING TYPES
// ============================================================================

export interface SystemMetrics {
  timestamp: string
  cpu: number // percentage
  ram: number // percentage
  disk: number // percentage
  queueDepth: number
  apiLatencyMs: number
  dbSlowQueriesCount: number
}

export interface BusinessMetrics {
  activeTenants: number
  dau: number // Daily Active Users
  mau: number // Monthly Active Users
  dormantTenants: number // Not logged in for 30+ days
  trialConversions: number
  churnedTenants: number
  timestamp: string
}

export interface HealthStatus {
  status: 'healthy' | 'degraded' | 'critical'
  message: string
  lastCheck: string
}

// ============================================================================
// SECURITY & AUDIT TYPES
// ============================================================================

export type AdminRole = 'super_admin' | 'admin' | 'operator' | 'viewer'
export type AuditAction =
  | 'create'
  | 'update'
  | 'delete'
  | 'login'
  | 'logout'
  | 'suspend'
  | 'reactivate'
  | 'export'
  | 'import'
  | 'role_change'
  | 'permission_change'
  | 'password_reset'
  | 'impersonate'

export interface AdminUser {
  id: string
  email: string
  name: string
  role: AdminRole
  status: 'active' | 'inactive' | 'disabled'
  lastLoginAt: string | null
  createdAt: string
  twoFactorEnabled: boolean
}

export interface AdminSession {
  id: string
  adminId: string
  adminEmail: string
  createdAt: string
  expiresAt: string
  ipAddress: string
  userAgent: string
  isActive: boolean
}

export interface AuditLog {
  id: string
  actor: {
    id: string
    email: string
    role: AdminRole
  }
  action: AuditAction
  target: {
    type: 'tenant' | 'subscription' | 'user' | 'backup' | 'admin'
    id: string
    name: string
  }
  details: Record<string, any>
  ipAddress: string
  userAgent: string
  timestamp: string
  status: 'success' | 'failed'
}

export interface RolePermission {
  id: string
  role: AdminRole
  permission: string
  description: string
}

// ============================================================================
// AUTOMATION RULES TYPES
// ============================================================================

export type RuleTrigger = 'schedule' | 'threshold' | 'event'
export type RuleAction = 'email' | 'suspend' | 'alert' | 'webhook' | 'backup'

export interface AutomationRule {
  id: string
  name: string
  description: string
  trigger: {
    type: RuleTrigger
    condition: string
    value?: number | string
  }
  action: {
    type: RuleAction
    target: string
    config: Record<string, any>
  }
  isActive: boolean
  createdAt: string
  lastTriggeredAt?: string | null
  executionCount: number
}

export interface RuleExecution {
  id: string
  ruleId: string
  ruleName: string
  triggeredAt: string
  completedAt: string
  status: 'success' | 'failed'
  error?: string | null
}

// ============================================================================
// ACTIVITY FEED TYPES
// ============================================================================

export type ActivityType =
  | 'tenant_created'
  | 'tenant_suspended'
  | 'tenant_reactivated'
  | 'billing_updated'
  | 'backup_completed'
  | 'backup_failed'
  | 'admin_login'
  // | 'invoice_issued'CreateTenantPayload
  // | 'invoice_paid'
  // | 'user_added'
  // | 'user_removed'
  // | 'plan_upgraded'
  // | 'plan_downgraded'

export interface ActivityFeedItem {
  id: string
  type: ActivityType
  actor?: {
    id: string
    name: string
  }
  subject: {
    type: string
    id: string
    name: string
  }
  description: string
  details?: Record<string, any>
  timestamp: string
}

// ============================================================================
// API RESPONSE TYPES
// ============================================================================

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  pageSize: number
  hasMore: boolean
}

export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: {
    code: string
    message: string
    details?: Record<string, any>
  }
  timestamp: string
}

// ============================================================================
// FILTER & SORT TYPES
// ============================================================================

export interface TableFilterConfig {
  status?: TenantStatus | string
  plan?: TenantPlan | string
  dateRange?: {
    start: string
    end: string
  }
  search?: string
}

export interface SortConfig {
  column: string
  direction: 'asc' | 'desc'
}

// ============================================================================
// NOTIFICATION TYPES
// ============================================================================

export interface Notification {
  id: string
  type: 'info' | 'warning' | 'error' | 'success'
  title: string
  message: string
  timestamp: string
  read: boolean
  actionUrl?: string | null
}
