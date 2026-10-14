/**
 * Shared Utilities
 * Common functions used throughout the application
 */

import { format, formatDistance, parseISO } from 'date-fns'

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

export function formatCurrency(
  amount: number,
  currency: string = 'USD'
): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency,
  }).format(amount)
}

export function formatDate(date: string | Date, formatStr: string = 'MMM dd, yyyy'): string {
  try {
    const d = typeof date === 'string' ? parseISO(date) : date
    return format(d, formatStr)
  } catch {
    return 'Invalid date'
  }
}

export function formatDateRelative(date: string | Date): string {
  try {
    const d = typeof date === 'string' ? parseISO(date) : date
    return formatDistance(d, new Date(), { addSuffix: true })
  } catch {
    return 'Invalid date'
  }
}

export function formatTime(date: string | Date, formatStr: string = 'HH:mm:ss'): string {
  try {
    const d = typeof date === 'string' ? parseISO(date) : date
    return format(d, formatStr)
  } catch {
    return 'Invalid time'
  }
}

export function getStatusBadgeVariant(
  status: string
): 'default' | 'secondary' | 'destructive' | 'outline'| 'paid' | 'refunded' | 'pending' | 'failed' {
  switch (status.toLowerCase()) {
    case 'active':
    case 'completed':
    case 'success':
      return 'default'
    case 'paid':
      return 'paid'
    case 'pending':
    case 'assigned':
    case 'in_progress':
      return 'pending'
    case 'refunded':
      return 'refunded'
    case 'suspended':
    case 'failed':
    case 'archived':
    case 'cancelled':
    case 'overdue':
      return 'failed'
    default:
      return 'outline'
  }
}

export function getPriorityColor(priority: string): string {
  switch (priority.toLowerCase()) {
    case 'urgent':
      return 'text-red-600 bg-red-50'
    case 'high':
      return 'text-orange-600 bg-orange-50'
    case 'medium':
      return 'text-blue-600 bg-blue-50'
    case 'low':
      return 'text-gray-600 bg-gray-50'
    default:
      return 'text-gray-600 bg-gray-50'
  }
}

export function getPriorityBadgeVariant(priority: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (priority.toLowerCase()) {
    case 'urgent':
      return 'destructive'
    case 'high':
      return 'secondary'
    case 'medium':
      return 'default'
    case 'low':
      return 'outline'
    default:
      return 'outline'
  }
}

export function truncateText(text: string, length: number = 50): string {
  return text.length > length ? text.slice(0, length) + '...' : text
}

export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\w-]/g, '')
    .replace(/--+/g, '-')
}

export function extractDomain(domain: string): string {
  try {
    const url = new URL(`https://${domain}`)
    return url.hostname
  } catch {
    return domain
  }
}

export function isValidEmail(email: string): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  return emailRegex.test(email)
}

export function debounce<T extends (...args: any[]) => any>(
  func: T,
  wait: number
): (...args: Parameters<T>) => void {
  let timeout: ReturnType<typeof setTimeout>

  return function executedFunction(...args: Parameters<T>) {
    const later = () => {
      clearTimeout(timeout)
      func(...args)
    }

    clearTimeout(timeout)
    timeout = setTimeout(later, wait)
  }
}

export function calculatePercentageChange(
  current: number,
  previous: number
): number {
  if (previous === 0) return 0
  return Math.round(((current - previous) / previous) * 100)
}

export function downloadCSV(data: any[], filename: string): void {
  const headers = Object.keys(data[0] || {})
  const csv = [
    headers.join(','),
    ...data.map((row) =>
      headers
        .map((header) => {
          const value = row[header]
          return typeof value === 'string' && value.includes(',')
            ? `"${value}"`
            : value
        })
        .join(',')
    ),
  ].join('\n')

  const blob = new Blob([csv], { type: 'text/csv' })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  window.URL.revokeObjectURL(url)
}

export function generateChartColors(): string[] {
  return [
    '#3b82f6', // blue
    '#10b981', // emerald
    '#f59e0b', // amber
    '#ef4444', // red
    '#8b5cf6', // violet
    '#ec4899', // pink
    '#06b6d4', // cyan
    '#14b8a6', // teal
  ]
}

export const TENANT_PLAN_FEATURES: Record<string, string[]> = {
  starter: [
    'Up to 50 users',
    'Email support',
    '50GB storage',
    'Monthly backups',
    'Basic analytics',
  ],
  professional: [
    'Up to 200 users',
    'Priority support',
    '500GB storage',
    'Daily backups',
    'Advanced analytics',
    'Custom domain',
    'API access',
  ],
  enterprise: [
    'Unlimited users',
    '24/7 support',
    'Unlimited storage',
    'Real-time backups',
    'Advanced analytics',
    'Custom domain',
    'API access',
    'Dedicated account manager',
    'SLA guarantee',
  ],
  custom: [
    'Custom configuration',
    'Dedicated support',
    'Custom features',
  ],
}
