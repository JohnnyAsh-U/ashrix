/**
 * Data Table Utilities & Columns
 * Shared components for displaying tabular data
 */

import type { ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import { getStatusBadgeVariant } from '@/lib/utils-control-plane'

export interface DataTableColumn<T> {
  key: string
  title: string
  render?: (value: any, row: T) => ReactNode
  sortable?: boolean
  width?: string
  align?: 'left' | 'center' | 'right'
}

// ========================================================================
// STATUS BADGE COMPONENTS
// ========================================================================

interface StatusBadgeProps {
  status: string
  label?: string
}

export function StatusBadge({ status, label }: StatusBadgeProps) {
  const displayLabel = label || status.replace(/_/g, ' ')
  return (
    <Badge variant={getStatusBadgeVariant(status)}>
      {displayLabel}
    </Badge>
  )
}

interface PlanBadgeProps {
  plan: string
}

export function PlanBadge({ plan }: PlanBadgeProps) {
  const colors: Record<string, any> = {
    starter: 'bg-blue-50 text-blue-700 border-blue-200',
    professional: 'bg-purple-50 text-purple-700 border-purple-200',
    enterprise: 'bg-gold-50 text-gold-700 border-gold-200',
    custom: 'bg-gray-50 text-gray-700 border-gray-200',
  }

  return (
    <div className={`px-3 py-1 rounded-md border text-sm font-medium ${colors[plan] || colors.custom}`}>
      {plan.charAt(0).toUpperCase() + plan.slice(1)}
    </div>
  )
}

interface PriorityBadgeProps {
  priority: 'low' | 'medium' | 'high' | 'urgent'
}

export function PriorityBadge({ priority }: PriorityBadgeProps) {
  const colors: Record<string, string> = {
    low: 'bg-gray-100 text-gray-800',
    medium: 'bg-blue-100 text-blue-800',
    high: 'bg-orange-100 text-orange-800',
    urgent: 'bg-red-100 text-red-800',
  }

  return (
    <Badge variant="secondary" className={colors[priority]}>
      {priority.charAt(0).toUpperCase() + priority.slice(1)}
    </Badge>
  )
}

// ========================================================================
// METRIC CARD COMPONENTS
// ========================================================================

interface MetricCardProps {
  label: string
  value: string | number
  icon?: ReactNode
  trend?: {
    value: number
    direction: 'up' | 'down'
  }
  onClick?: () => void
}

export function MetricCard({ label, value, icon, trend, onClick }: MetricCardProps) {
  return (
    <div
      onClick={onClick}
      className={`p-6 rounded-lg border bg-white ${
        onClick ? 'cursor-pointer hover:border-gray-400' : ''
      }`}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-gray-600 font-medium">{label}</p>
          <p className="text-2xl font-bold text-gray-900 mt-2">{value}</p>
          {trend && (
            <p className={`text-sm mt-2 ${trend.direction === 'up' ? 'text-green-600' : 'text-red-600'}`}>
              {trend.direction === 'up' ? '↑' : '↓'} {trend.value}% vs last period
            </p>
          )}
        </div>
        {icon && <div className="text-gray-400">{icon}</div>}
      </div>
    </div>
  )
}

// ========================================================================
// STAT DISPLAY COMPONENTS
// ========================================================================

interface StatItemProps {
  label: string
  value: string | number
  unit?: string
  isMoney?: boolean
  isPercentage?: boolean
}

export function StatItem({ label, value, unit, isMoney, isPercentage }: StatItemProps) {
  let displayValue = value

  if (isMoney) {
    displayValue = typeof value === 'number' ? `$${value.toLocaleString()}` : value
  } else if (isPercentage) {
    displayValue = typeof value === 'number' ? `${value}%` : value
  }

  return (
    <div>
      <p className="text-sm text-gray-600">{label}</p>
      <p className="text-lg font-semibold text-gray-900 mt-1">
        {displayValue}
        {unit && <span className="text-sm text-gray-500 ml-1">{unit}</span>}
      </p>
    </div>
  )
}

// ========================================================================
// ACTIVITY ITEM COMPONENT
// ========================================================================

interface ActivityItemProps {
  icon?: ReactNode
  title: string
  description?: string
  timestamp: string
  status?: 'success' | 'error' | 'info'
}

export function ActivityItem({ icon, title, description, timestamp, status }: ActivityItemProps) {
  const statusColors = {
    success: 'bg-green-50 text-green-700',
    error: 'bg-red-50 text-red-700',
    info: 'bg-blue-50 text-blue-700',
  }

  return (
    <div className="flex gap-4 p-3 rounded-lg border border-gray-100 hover:bg-gray-50">
      {icon && (
        <div className={`p-2 rounded ${status ? statusColors[status] : 'bg-gray-100 text-gray-600'}`}>
          {icon}
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-gray-900">{title}</p>
        {description && <p className="text-sm text-gray-600 mt-1">{description}</p>}
        <p className="text-xs text-gray-500 mt-1">{timestamp}</p>
      </div>
    </div>
  )
}

// ========================================================================
// EMPTY STATE COMPONENT
// ========================================================================

interface EmptyStateProps {
  title: string
  description?: string
  icon?: ReactNode
  action?: {
    label: string
    onClick: () => void
  }
}

export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      {icon && <div className="text-gray-400 mb-4 text-4xl">{icon}</div>}
      <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
      {description && <p className="text-gray-600 text-sm mt-1">{description}</p>}
      {action && (
        <button
          onClick={action.onClick}
          className="mt-4 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700"
        >
          {action.label}
        </button>
      )}
    </div>
  )
}

// ========================================================================
// LOADING SKELETON
// ========================================================================

export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-2">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-12 bg-gray-200 rounded animate-pulse" />
      ))}
    </div>
  )
}

export function CardSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="p-6 rounded-lg border bg-white">
          <div className="h-4 bg-gray-200 rounded w-1/2 mb-3 animate-pulse" />
          <div className="h-8 bg-gray-200 rounded w-3/4 animate-pulse" />
        </div>
      ))}
    </div>
  )
}

// ========================================================================
// INFO BOX
// ========================================================================

interface InfoBoxProps {
  type: 'info' | 'warning' | 'error' | 'success'
  title?: string
  message: string
  onClose?: () => void
}

export function InfoBox({ type, title, message, onClose }: InfoBoxProps) {
  const typeStyles = {
    info: 'bg-blue-50 border-blue-200 text-blue-900',
    warning: 'bg-yellow-50 border-yellow-200 text-yellow-900',
    error: 'bg-red-50 border-red-200 text-red-900',
    success: 'bg-green-50 border-green-200 text-green-900',
  }

  return (
    <div className={`p-4 rounded-lg border ${typeStyles[type]}`}>
      <div className="flex items-start justify-between">
        <div>
          {title && <p className="font-semibold text-sm">{title}</p>}
          <p className="text-sm mt-1">{message}</p>
        </div>
        {onClose && (
          <button onClick={onClose} className="ml-4 text-lg opacity-50 hover:opacity-100">
            ×
          </button>
        )}
      </div>
    </div>
  )
}
