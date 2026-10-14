import { format } from 'date-fns/format'
import { parseISO } from 'date-fns/parseISO'


interface StatItemProps {
  label: string
  value: string | number
  unit?: string
  isMoney?: boolean
  isPercentage?: boolean
}


export function formatTime(date: string | Date, formatStr: string = 'HH:mm:ss'): string {
  try {
    const d = typeof date === 'string' ? parseISO(date) : date
    return format(d, formatStr)
  } catch {
    return 'Invalid time'
  }
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


