/**
 * API Service Layer
 * Handles all data fetching and mutations for the control plane
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
  RuleExecution,
  PaginatedResponse,
} from '@/types'

import {
  generateMockTenant,
  generateMockTenantDetail,
  generateMockSubscription,
  generateMockInvoice,
  generateMockBackup,
  generateMockEmail,
  generateMockAdminUser,
  generateMockAuditLog,
  generateMockActivityFeedItem,
  generateMockSystemMetrics,
  generateMockBusinessMetrics,
  generateMockCoupon,
  generateMockPlan,
  generateMockAutomationRule,
} from './mock-data'

// Simulate API delay
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

class ApiService {
  private baseDelay = 300 // ms
  private tenants: Tenant[] = []
  private subscriptions: Subscription[] = []
  private invoices: Invoice[] = []
  private backups: Backup[] = []
  private emails: Email[] = []
  private adminUsers: AdminUser[] = []
  private auditLogs: AuditLog[] = []

  constructor() {
    this.initializeMockData()
  }

  private initializeMockData() {
    // Generate 50 tenants
    for (let i = 0; i < 50; i++) {
      this.tenants.push(generateMockTenant(i))
    }

    // Generate subscriptions
    for (let i = 0; i < 50; i++) {
      const tenant = this.tenants[i]
      this.subscriptions.push(
        generateMockSubscription(i, String(tenant.id), tenant.name || '')
      )
    }

    // Generate invoices
    for (let i = 0; i < 200; i++) {
      const tenant = this.tenants[i % this.tenants.length]
      this.invoices.push(generateMockInvoice(i, String(tenant.id), tenant.name || ''  ))
    }

    // Generate backups
    for (let i = 0; i < 150; i++) {
      const tenant = this.tenants[i % this.tenants.length]
      this.backups.push(generateMockBackup(i, String(tenant.id), tenant.name || ''))
    }

    // Generate emails
    for (let i = 0; i < 100; i++) {
      this.emails.push(generateMockEmail(i))
    }

    // Generate admin users
    for (let i = 0; i < 10; i++) {
      this.adminUsers.push(generateMockAdminUser(i))
    }

    // Generate audit logs
    for (let i = 0; i < 300; i++) {
      this.auditLogs.push(generateMockAuditLog(i))
    }
  }

  // ========================================================================
  // TENANT ENDPOINTS
  // ========================================================================

  async getTenants(
    page: number = 1,
    pageSize: number = 20,
    search?: string,
    status?: string
  ): Promise<PaginatedResponse<Tenant>> {
    await delay(this.baseDelay)

    let filtered = [...this.tenants]

    if (search) {
      const query = search.toLowerCase()
      filtered = filtered.filter(
        (t) =>
          (t.name || '').toLowerCase().includes(query) ||
          (t.email || '').toLowerCase().includes(query) ||
          (t.domain || '').toLowerCase().includes(query)
      )
    }

    if (status && status !== 'all') {
      filtered = filtered.filter((t) => t.status === status)
    }

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: filtered.slice(start, end),
      total: filtered.length,
      page,
      pageSize,
      hasMore: end < filtered.length,
    }
  }

  async getTenantById(tenantId: string): Promise<TenantDetail> {
    await delay(this.baseDelay)

    const tenant = this.tenants.find((t) => t.id === Number(tenantId))
    if (!tenant) throw new Error('Tenant not found')

    return generateMockTenantDetail(tenant)
  }

  async updateTenantStatus(
    tenantId: string,
    status: string,
    reason?: string
  ): Promise<Tenant> {
    await delay(this.baseDelay + 200)

    const tenant = this.tenants.find((t) => t.id === Number(tenantId))
    if (!tenant) throw new Error('Tenant not found')

    tenant.status = status as any
    if (status === 'suspended') {
      tenant.suspendedAt = new Date().toISOString()
      tenant.suspendedReason = reason
    }
    tenant.updatedAt = new Date().toISOString()

    return tenant
  }

  async impersonateTenant(tenantId: string): Promise<{ token: string; redirectUrl: string }> {
    await delay(this.baseDelay + 100)

    const tenant = this.tenants.find((t) => t.id === Number(tenantId))
    if (!tenant) throw new Error('Tenant not found')

    return {
      token: `impersonate_${tenantId}_${Date.now()}`,
      redirectUrl: `https://${tenant.subdomain}.localhost:3000/dashboard`,
    }
  }

  async changeTenantPlan(tenantId: string, newPlan: string): Promise<Tenant> {
    await delay(this.baseDelay + 200)

    const tenant = this.tenants.find((t) => t.id === Number(tenantId))
    if (!tenant) throw new Error('Tenant not found')

    tenant.plan = newPlan as any
    tenant.updatedAt = new Date().toISOString()

    return tenant
  }

  async deleteTenant(tenantId: string): Promise<{ success: boolean }> {
    await delay(this.baseDelay + 300)

    const index = this.tenants.findIndex((t) => t.id === Number(tenantId))
    if (index === -1) throw new Error('Tenant not found')

    this.tenants.splice(index, 1)
    return { success: true }
  }

  // ========================================================================
  // SUBSCRIPTION ENDPOINTS
  // ========================================================================

  async getSubscriptions(
    page: number = 1,
    pageSize: number = 20
  ): Promise<PaginatedResponse<Subscription>> {
    await delay(this.baseDelay)

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: this.subscriptions.slice(start, end),
      total: this.subscriptions.length,
      page,
      pageSize,
      hasMore: end < this.subscriptions.length,
    }
  }

  async updateSubscriptionPlan(
    subscriptionId: string,
    newPlan: string
  ): Promise<Subscription> {
    await delay(this.baseDelay + 200)

    const sub = this.subscriptions.find((s) => s.id === subscriptionId)
    if (!sub) throw new Error('Subscription not found')

    sub.plan = newPlan as any
    return sub
  }

  // ========================================================================
  // INVOICE ENDPOINTS
  // ========================================================================

  async getInvoices(
    page: number = 1,
    pageSize: number = 20,
    status?: string
  ): Promise<PaginatedResponse<Invoice>> {
    await delay(this.baseDelay)

    let filtered = [...this.invoices]

    if (status && status !== 'all') {
      filtered = filtered.filter((i) => i.status === status)
    }

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: filtered.slice(start, end),
      total: filtered.length,
      page,
      pageSize,
      hasMore: end < filtered.length,
    }
  }

  async markInvoicePaid(invoiceId: string): Promise<Invoice> {
    await delay(this.baseDelay + 150)

    const invoice = this.invoices.find((i) => i.id === invoiceId)
    if (!invoice) throw new Error('Invoice not found')

    invoice.status = 'paid'
    invoice.paidDate = new Date().toISOString()

    return invoice
  }

  async issueRefund(invoiceId: string, amount: number): Promise<Invoice> {
    await delay(this.baseDelay + 200)

    const invoice = this.invoices.find((i) => i.id === invoiceId)
    if (!invoice) throw new Error('Invoice not found')

    invoice.amount -= amount
    return invoice
  }

  // ========================================================================
  // BACKUP ENDPOINTS
  // ========================================================================

  async getBackups(
    tenantId?: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<PaginatedResponse<Backup>> {
    await delay(this.baseDelay)

    let filtered = [...this.backups]

    if (tenantId) {
      filtered = filtered.filter((b) => b.tenantId === tenantId)
    }

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: filtered.slice(start, end),
      total: filtered.length,
      page,
      pageSize,
      hasMore: end < filtered.length,
    }
  }

  async createBackup(tenantId: string): Promise<Backup> {
    await delay(this.baseDelay + 500)

    const tenant = this.tenants.find((t) => t.id == Number(tenantId))
    if (!tenant) throw new Error('Tenant not found')

    const backup: Backup = {
      id: `backup_${Date.now()}`,
      tenantId,
      tenantName: tenant.name ?? '',
      type: 'manual',
      status: 'completed',
      sizeGB: Math.floor(Math.random() * 50) + 1,
      createdAt: new Date().toISOString(),
      completedAt: new Date().toISOString(),
      expiresAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
      retentionDays: 30,
    }

    this.backups.push(backup)
    return backup
  }

  async restoreBackup(backupId: string): Promise<{ jobId: string; status: string }> {
    await delay(this.baseDelay + 300)

    const backup = this.backups.find((b) => b.id === backupId)
    if (!backup) throw new Error('Backup not found')

    return {
      jobId: `restore_${Date.now()}`,
      status: 'started',
    }
  }

  // ========================================================================
  // EMAIL/SUPPORT ENDPOINTS
  // ========================================================================

  async getEmails(
    inbox?: string,
    status?: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<PaginatedResponse<Email>> {
    await delay(this.baseDelay)

    let filtered = [...this.emails]

    if (inbox && inbox !== 'all') {
      filtered = filtered.filter((e) => e.inboxType === inbox)
    }

    if (status && status !== 'all') {
      filtered = filtered.filter((e) => e.status === status)
    }

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: filtered.slice(start, end),
      total: filtered.length,
      page,
      pageSize,
      hasMore: end < filtered.length,
    }
  }

  async assignEmail(emailId: string, operatorId: string): Promise<Email> {
    await delay(this.baseDelay + 150)

    const email = this.emails.find((e) => e.id === emailId)
    if (!email) throw new Error('Email not found')

    email.assignedTo = operatorId
    email.status = 'assigned'

    return email
  }

  async resolveEmail(emailId: string): Promise<Email> {
    await delay(this.baseDelay + 150)

    const email = this.emails.find((e) => e.id === emailId)
    if (!email) throw new Error('Email not found')

    email.status = 'resolved'
    return email
  }

  // ========================================================================
  // ADMIN USER ENDPOINTS
  // ========================================================================

  async getAdminUsers(): Promise<AdminUser[]> {
    await delay(this.baseDelay)
    return [...this.adminUsers]
  }

  async updateAdminRole(adminId: string, newRole: string): Promise<AdminUser> {
    await delay(this.baseDelay + 150)

    const admin = this.adminUsers.find((a) => a.id === adminId)
    if (!admin) throw new Error('Admin user not found')

    admin.role = newRole as any
    return admin
  }

  async revokeAdminSession(_sessionId: string): Promise<{ success: boolean }> {
    await delay(this.baseDelay + 100)
    return { success: true }
  }

  // ========================================================================
  // AUDIT LOG ENDPOINTS
  // ========================================================================

  async getAuditLogs(
    page: number = 1,
    pageSize: number = 20,
    filters?: Record<string, any>
  ): Promise<PaginatedResponse<AuditLog>> {
    await delay(this.baseDelay)

    let filtered = [...this.auditLogs]

    if (filters?.action) {
      filtered = filtered.filter((a) => a.action === filters.action)
    }

    if (filters?.targetType) {
      filtered = filtered.filter((a) => a.target.type === filters.targetType)
    }

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: filtered.slice(start, end),
      total: filtered.length,
      page,
      pageSize,
      hasMore: end < filtered.length,
    }
  }

  // ========================================================================
  // METRICS ENDPOINTS
  // ========================================================================

  async getSystemMetrics(): Promise<SystemMetrics> {
    await delay(this.baseDelay)
    return generateMockSystemMetrics()
  }

  async getBusinessMetrics(): Promise<BusinessMetrics> {
    await delay(this.baseDelay)
    return generateMockBusinessMetrics()
  }

  async getRevenueData(): Promise<
    Array<{ month: string; revenue: number; signups: number }>
  > {
    await delay(this.baseDelay)

    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec',
    ]
    return months.map((month) => ({
      month,
      revenue: Math.floor(Math.random() * 150000) + 50000,
      signups: Math.floor(Math.random() * 300) + 50,
    }))
  }

  async getActivityFeed(page: number = 1, pageSize: number = 10): Promise<
    PaginatedResponse<ActivityFeedItem>
  > {
    await delay(this.baseDelay)

    const items = Array.from({ length: 100 }, (_, i) =>
      generateMockActivityFeedItem(i)
    )

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: items.slice(start, end),
      total: items.length,
      page,
      pageSize,
      hasMore: end < items.length,
    }
  }

  // ========================================================================
  // BILLING MODULE ENDPOINTS
  // ========================================================================

  async getCoupons(page: number = 1, pageSize: number = 20): Promise<
    PaginatedResponse<Coupon>
  > {
    await delay(this.baseDelay)

    const coupons = Array.from({ length: 50 }, (_, i) =>
      generateMockCoupon(i)
    )

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: coupons.slice(start, end),
      total: coupons.length,
      page,
      pageSize,
      hasMore: end < coupons.length,
    }
  }

  async getPlans(): Promise<Plan[]> {
    await delay(this.baseDelay)
    return Array.from({ length: 4 }, (_, i) => generateMockPlan(i))
  }

  // ========================================================================
  // AUTOMATION RULES ENDPOINTS
  // ========================================================================

  async getAutomationRules(page: number = 1, pageSize: number = 20): Promise<
    PaginatedResponse<AutomationRule>
  > {
    await delay(this.baseDelay)

    const rules = Array.from({ length: 50 }, (_, i) =>
      generateMockAutomationRule(i)
    )

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: rules.slice(start, end),
      total: rules.length,
      page,
      pageSize,
      hasMore: end < rules.length,
    }
  }

  async createAutomationRule(rule: Partial<AutomationRule>): Promise<AutomationRule> {
    await delay(this.baseDelay + 200)

    const newRule: AutomationRule = {
      id: `rule_${Date.now()}`,
      name: rule.name || 'Untitled Rule',
      description: rule.description || '',
      trigger: rule.trigger || { type: 'event', condition: '' },
      action: rule.action || { type: 'email', target: '', config: {} },
      isActive: true,
      createdAt: new Date().toISOString(),
      executionCount: 0,
    }

    return newRule
  }

  async updateAutomationRule(ruleId: string, updates: Partial<AutomationRule>): Promise<AutomationRule> {
    await delay(this.baseDelay + 150)

    return {
      id: ruleId,
      name: updates.name || 'Updated Rule',
      description: updates.description || '',
      trigger: updates.trigger || { type: 'event', condition: '' },
      action: updates.action || { type: 'email', target: '', config: {} },
      isActive: updates.isActive ?? true,
      createdAt: new Date().toISOString(),
      executionCount: 0,
    }
  }

  async toggleAutomationRule(ruleId: string, isActive: boolean): Promise<AutomationRule> {
    await delay(this.baseDelay + 100)

    return {
      id: ruleId,
      name: 'Rule',
      description: '',
      trigger: { type: 'event', condition: '' },
      action: { type: 'email', target: '', config: {} },
      isActive,
      createdAt: new Date().toISOString(),
      executionCount: 0,
    }
  }

  async getRuleExecutions(ruleId: string, page: number = 1, pageSize: number = 20): Promise<
    PaginatedResponse<RuleExecution>
  > {
    await delay(this.baseDelay)

    const executions: RuleExecution[] = Array.from({ length: 100 }, (_, i) => ({
      id: `exec_${i}`,
      ruleId,
      ruleName: 'Sample Rule',
      triggeredAt: new Date(Date.now() - i * 60 * 60 * 1000).toISOString(),
      completedAt: new Date(Date.now() - i * 60 * 60 * 1000 + 30000).toISOString(),
      status: Math.random() > 0.1 ? 'success' : 'failed',
      error: Math.random() > 0.1 ? null : 'Webhook timeout',
    }))

    const start = (page - 1) * pageSize
    const end = start + pageSize

    return {
      data: executions.slice(start, end),
      total: executions.length,
      page,
      pageSize,
      hasMore: end < executions.length,
    }
  }
}

// Export singleton instance
export const apiService = new ApiService()
