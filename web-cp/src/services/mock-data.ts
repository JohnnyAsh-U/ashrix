/**
 * Mock Data Generators
 * Realistic data for development and testing
 */

import type {
  Tenant,
  TenantDetail,
  Subscription,
  Invoice,
  Backup,
  Email,
  AdminUser,
  AuditLog,
  ActivityFeedItem,
  SystemMetrics,
  BusinessMetrics,
  Coupon,
  Plan,
  AutomationRule,
} from '@/types'

// Mock tenant names
const TENANT_NAMES = [
  'Grace Community Church',
  'Holy Trinity Parish',
  'New Life Assembly',
  'Bethel Missionary',
  'Cornerstone Fellowship',
  'Emanuel Baptist',
  'Kingdom Harvest Church',
  'Shiloh Pentecostal',
  'Solid Rock Ministry',
  'Victory Christian Center',
  'Living Waters Community',
  'First Apostolic Church',
  'Redeemer Chapel',
  'Cross Way Alliance',
  'Advent Reformed',
  'Mount Zion Tabernacle',
  'Faith Family Fellowship',
  'Gospel Light Mission',
  'Blessed Hope Assembly',
  'Restoration Church',
]

const DOMAINS = [
  'gracecc.org',
  'holytrinityparish.org',
  'newlifeassembly.com',
  'bethelmissionary.org',
  'cornerstonefelowship.com',
  'emanuelbaptist.org',
  'kingdomharvest.church',
  'shilohpentecostal.org',
  'solidrockministry.com',
  'victorycc.org',
]

const STATUSES = ['active', 'suspended', 'trial', 'archived'] as const
const PLANS = ['starter', 'professional', 'enterprise', 'custom'] as const

export function generateMockTenant(index: number): Tenant {
  const status =
    STATUSES[Math.floor(Math.random() * STATUSES.length)]
  const plan = PLANS[Math.floor(Math.random() * PLANS.length)]
  const name = TENANT_NAMES[index % TENANT_NAMES.length]
  const domain = DOMAINS[index % DOMAINS.length]

  return {
  id: index,
  name: `${name} ${index > 0 ? `- Branch ${Math.floor(index / 2)}` : ''}`.trim(),
  email: `admin@${domain}`,
  domain: `${name.toLowerCase().replace(/\s+/g, '-')}-${index}.${domain}`,
  subdomain: `${name.toLowerCase().replace(/\s+/g, '-')}-${index}`,
  status,
  plan,
  usersCount: Math.floor(Math.random() * 500) + 5,
  storageUsedGB: Math.floor(Math.random() * 50) + 1,
  storageQuotaGB: plan === 'starter' ? 50 : plan === 'professional' ? 500 : 2000,
  lastLoginAt: Math.random() > 0.2
    ? new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
    : null,
  createdAt: new Date(
    Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000
  ).toISOString(),
  updatedAt: new Date().toISOString(),
  suspendedAt: status === 'suspended'
    ? new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
    : null,
  suspendedReason: status === 'suspended'
    ? Math.random() > 0.5
      ? 'Payment overdue'
      : 'Policy violation'
    : null,
  church_name: '',
  domains: [],
  is_active: false,
  church_count: 0,
  member_count: 0,
  custom_logo: false,
  phone: ''
}
}

export function generateMockTenantDetail(tenant: Tenant): TenantDetail {
  return {
    ...tenant,
    adminUser: {
      id: `user_${tenant.id}`,
      name: 'Pastor John Smith',
      email: tenant.email || `admin@${tenant.domain}`,
    },
    billingEmail: `billing@${tenant.domain}`,
    supportEmail: `support@${tenant.domain}`,
    organization: tenant.name || `Organization ${tenant.id}`,
    website: `https://${tenant.domain}`,
    databaseSchema: `schema_${tenant.id}`,
    backupRetentionDays: tenant.plan === 'starter' ? 7 : tenant.plan === 'professional' ? 30 : 90,
    maxApiCalls: tenant.plan === 'starter' ? 10000 : tenant.plan === 'professional' ? 100000 : 1000000,
    customDomain: Math.random() > 0.7 ? `custom.${tenant.domain}` : null,
  }
}

export function generateMockSubscription(
  index: number,
  tenantId: string,
  tenantName: string
): Subscription {
  const plan = PLANS[Math.floor(Math.random() * PLANS.length)]
  const status = Math.random() > 0.1 ? 'active' : Math.random() > 0.5 ? 'cancelled' : 'suspended'

  return {
    id: `sub_${index}`,
    tenantId,
    tenantName,
    plan,
    status,
    currentPeriodStart: new Date(
      Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
    ).toISOString(),
    currentPeriodEnd: new Date(
      Date.now() + Math.random() * 30 * 24 * 60 * 60 * 1000
    ).toISOString(),
    cancelledAt:
      status === 'cancelled'
        ? new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
        : null,
    autoRenew: status === 'active' && Math.random() > 0.1,
    createdAt: new Date(
      Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000
    ).toISOString(),
  }
}

export function generateMockInvoice(
  index: number,
  tenantId: string,
  tenantName: string
): Invoice {
  const amount = Math.floor(Math.random() * 5000) + 99
  const status =
    Math.random() > 0.8
      ? 'paid'
      : Math.random() > 0.6
        ? 'pending'
        : Math.random() > 0.4
          ? 'failed'
          : 'overdue'

  const issuedDate = new Date(
    Date.now() - Math.random() * 90 * 24 * 60 * 60 * 1000
  )

  return {
    id: `inv_${index}`,
    tenantId,
    tenantName,
    amount,
    currency: 'USD',
    status,
    dueDate: new Date(issuedDate.getTime() + 30 * 24 * 60 * 60 * 1000).toISOString(),
    issuedDate: issuedDate.toISOString(),
    paidDate:
      status === 'paid'
        ? new Date(issuedDate.getTime() + Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
        : null,
    description: `Monthly subscription - ${['Starter', 'Professional', 'Enterprise'][Math.floor(Math.random() * 3)]} Plan`,
    itemCount: Math.floor(Math.random() * 5) + 1,
  }
}

export function generateMockBackup(
  index: number,
  tenantId: string,
  tenantName: string
): Backup {
  const statusOptions = ['pending', 'in_progress', 'completed', 'failed'] as const
  const status = statusOptions[Math.floor(Math.random() * 4)]
  const createdAt = new Date(
    Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
  )

  return {
    id: `backup_${index}`,
    tenantId,
    tenantName,
    type: Math.random() > 0.5 ? 'scheduled' : 'manual',
    status,
    sizeGB: Math.floor(Math.random() * 50) + 1,
    createdAt: createdAt.toISOString(),
    completedAt:
      status === 'completed' || status === 'failed'
        ? new Date(createdAt.getTime() + Math.random() * 60 * 60 * 1000).toISOString()
        : null,
    expiresAt: new Date(
      createdAt.getTime() + 30 * 24 * 60 * 60 * 1000
    ).toISOString(),
    retentionDays: 30,
    failureReason: status === 'failed' ? 'Timeout during backup' : null,
  }
}

export function generateMockEmail(index: number): Email {
  const inboxTypes = ['support', 'billing', 'sales'] as const
  const inboxType = inboxTypes[Math.floor(Math.random() * 3)]
  const priorityOptions = ['low', 'medium', 'high', 'urgent'] as const
  const priority = priorityOptions[Math.floor(Math.random() * 4)]
  const statusOptions = ['open', 'assigned', 'pending_response', 'resolved'] as const
  const status = statusOptions[Math.floor(Math.random() * 4)]
  const receivedAt = new Date(
    Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
  )

  return {
    id: `email_${index}`,
    inboxType,
    subject:
      inboxType === 'support'
        ? `Issue with user permissions - ID ${Math.floor(Math.random() * 1000)}`
        : inboxType === 'billing'
          ? `Invoice dispute - Invoice ${Math.floor(Math.random() * 10000)}`
          : `New integration inquiry`,
    from: `customer${index}@example.com`,
    to: `${inboxType}@ecclesix.io`,
    body: 'We need assistance with our account setup and configuration.',
    priority,
    status,
    assignedTo:
      status !== 'open'
        ? `operator_${Math.floor(Math.random() * 5)}`
        : null,
    tags: [inboxType, priority],
    slaDeadline:
      priority === 'urgent'
        ? new Date(Date.now() + 4 * 60 * 60 * 1000).toISOString()
        : priority === 'high'
          ? new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
          : null,
    slaBreached: Math.random() > 0.9,
    receivedAt: receivedAt.toISOString(),
    responseCount: Math.floor(Math.random() * 5),
    lastResponseAt:
      status !== 'open'
        ? new Date(receivedAt.getTime() + Math.random() * 24 * 60 * 60 * 1000).toISOString()
        : null,
  }
}

export function generateMockAdminUser(index: number): AdminUser {
  const roles = ['super_admin', 'admin', 'operator', 'viewer'] as const

  return {
    id: `admin_${index}`,
    email: `admin${index}@ecclesix.io`,
    name: [
      'Sarah Johnson',
      'Michael Chen',
      'Emma Williams',
      'David Martinez',
      'Jessica Brown',
    ][index % 5],
    role: roles[index % 4],
    status: Math.random() > 0.1 ? 'active' : 'inactive',
    lastLoginAt:
      Math.random() > 0.2
        ? new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000).toISOString()
        : null,
    createdAt: new Date(
      Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000
    ).toISOString(),
    twoFactorEnabled: Math.random() > 0.4,
  }
}

export function generateMockAuditLog(index: number): AuditLog {
  const actions = [
    'create',
    'update',
    'delete',
    'login',
    'logout',
    'suspend',
    'reactivate',
    'export',
    'import',
    'role_change',
    'password_reset',
    'impersonate',
  ] as const
  const targetTypes = ['tenant', 'subscription', 'user', 'backup', 'admin'] as const

  return {
    id: `audit_${index}`,
    actor: {
      id: `admin_${Math.floor(Math.random() * 5)}`,
      email: `admin${Math.floor(Math.random() * 5)}@ecclesix.io`,
      role: ['super_admin', 'admin', 'operator', 'viewer'][
        Math.floor(Math.random() * 4)
      ] as any,
    },
    action: actions[Math.floor(Math.random() * actions.length)],
    target: {
      type: targetTypes[Math.floor(Math.random() * targetTypes.length)],
      id: `target_${Math.floor(Math.random() * 1000)}`,
      name: TENANT_NAMES[index % TENANT_NAMES.length],
    },
    details: {
      changesSummary: 'Status updated from active to suspended',
    },
    ipAddress: `192.168.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
    timestamp: new Date(
      Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
    ).toISOString(),
    status: Math.random() > 0.05 ? 'success' : 'failed',
  }
}

export function generateMockActivityFeedItem(index: number): ActivityFeedItem {
  const types = [
    'tenant_created',
    'tenant_suspended',
    'tenant_reactivated',
    'billing_updated',
    'backup_completed',
    'admin_login',
  ] as const

  const type = types[Math.floor(Math.random() * types.length)] as ActivityFeedItem['type']

  const descriptions: Record<string, string> = {
    tenant_created: 'New tenant registered',
    tenant_suspended: 'Tenant account suspended',
    tenant_reactivated: 'Tenant account reactivated',
    billing_updated: 'Billing information updated',
    backup_completed: 'Backup completed successfully',
    admin_login: 'Admin logged in',
  }

  return {
    id: `activity_${index}`,
    type,
    actor: {
      id: `admin_${Math.floor(Math.random() * 5)}`,
      name: 'Admin System',
    },
    subject: {
      type: 'tenant',
      id: `tenant_${Math.floor(Math.random() * 100)}`,
      name: TENANT_NAMES[index % TENANT_NAMES.length] || `Tenant ${index}`,
    },
    description: descriptions[type] ?? 'Activity recorded',
    timestamp: new Date(
      Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
    ).toISOString(),
  }
}

export function generateMockSystemMetrics(): SystemMetrics {
  return {
    timestamp: new Date().toISOString(),
    cpu: Math.floor(Math.random() * 80) + 10,
    ram: Math.floor(Math.random() * 85) + 15,
    disk: Math.floor(Math.random() * 70) + 20,
    queueDepth: Math.floor(Math.random() * 500) + 50,
    apiLatencyMs: Math.floor(Math.random() * 200) + 50,
    dbSlowQueriesCount: Math.floor(Math.random() * 20),
  }
}

export function generateMockBusinessMetrics(): BusinessMetrics {
  return {
    activeTenants: Math.floor(Math.random() * 5000) + 1000,
    dau: Math.floor(Math.random() * 50000) + 10000,
    mau: Math.floor(Math.random() * 80000) + 40000,
    dormantTenants: Math.floor(Math.random() * 200) + 50,
    trialConversions: Math.floor(Math.random() * 100) + 20,
    churnedTenants: Math.floor(Math.random() * 50) + 5,
    timestamp: new Date().toISOString(),
  }
}

export function generateMockCoupon(index: number): Coupon {
  return {
    id: `coupon_${index}`,
    code: `CHURCH${index}`,
    description: `Discount coupon for churches and nonprofits`,
    discountType: Math.random() > 0.5 ? 'percentage' : 'fixed',
    discountValue: Math.random() > 0.5 ? Math.floor(Math.random() * 50) + 5 : Math.floor(Math.random() * 500) + 50,
    maxUses: Math.floor(Math.random() * 1000) + 100,
    currentUses: Math.floor(Math.random() * 500),
    isActive: Math.random() > 0.1,
    expiresAt:
      Math.random() > 0.5
        ? new Date(Date.now() + Math.random() * 180 * 24 * 60 * 60 * 1000).toISOString()
        : null,
    createdAt: new Date(
      Date.now() - Math.random() * 180 * 24 * 60 * 60 * 1000
    ).toISOString(),
  }
}

export function generateMockPlan(index: number): Plan {
  const planNames = ['starter', 'professional', 'enterprise', 'custom'] as const

  return {
    id: `plan_${index}`,
    name: planNames[index % 4],
    displayName: ['Starter', 'Professional', 'Enterprise', 'Custom'][index % 4],
    description: [
      'Perfect for small organizations',
      'For growing communities',
      'For large organizations',
      'Custom solution',
    ][index % 4],
    monthlyPrice: [29, 99, 299, 0][index % 4],
    annualPrice: [290, 990, 2990, 0][index % 4],
    features: [
      'Up to 50 users',
      'Email support',
      'Monthly backups',
      'Basic analytics',
      ...(index > 0 ? ['Up to 200 users', '24/7 support', 'Daily backups'] : []),
      ...(index > 1 ? ['Unlimited users', 'Priority support', 'Real-time backups', 'Advanced analytics'] : []),
    ],
    userLimit: [50, 200, 0, 0][index % 4],
    storageGBLimit: [50, 500, 5000, 0][index % 4],
    apiCallsLimit: [10000, 100000, 1000000, 0][index % 4],
    isActive: true,
  }
}

export function generateMockAutomationRule(index: number): AutomationRule {
  const triggers = ['schedule', 'threshold', 'event'] as const
  const actions = ['email', 'suspend', 'alert', 'webhook', 'backup'] as const

  return {
    id: `rule_${index}`,
    name: [
      'Suspend overdue payments',
      'Alert on high storage',
      'Daily backup schedule',
      'Notify dormant tenants',
      'Failed payment retry',
    ][index % 5],
    description: [
      'Suspend tenant if invoice overdue 7 days',
      'Send alert if storage exceeds 90%',
      'Run daily backups at 2 AM',
      'Email tenants inactive 30+ days',
      'Retry failed payments',
    ][index % 5],
    trigger: {
      type: triggers[index % 3],
      condition: ['invoice_overdue', 'storage_threshold', 'schedule', 'inactivity'][index % 4],
      value: index % 2 === 0 ? 7 : 90,
    },
    action: {
      type: actions[index % 5],
      target: [
        'tenant',
        'admin',
        'system',
        'webhook_endpoint',
      ][index % 4],
      config: {
        template: 'overdue_notice',
        retryCount: 3,
      },
    },
    isActive: Math.random() > 0.1,
    createdAt: new Date(
      Date.now() - Math.random() * 180 * 24 * 60 * 60 * 1000
    ).toISOString(),
    lastTriggeredAt:
      Math.random() > 0.3
        ? new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
        : null,
    executionCount: Math.floor(Math.random() * 500),
  }
}
