/**
 * Shared Filter Components
 * Reusable filters for tables and lists
 */

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Search, X, Filter } from 'lucide-react'

// ========================================================================
// SEARCH BAR COMPONENT
// ========================================================================

interface SearchBarProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  onClear?: () => void
}

export function SearchBar({
  value,
  onChange,
  placeholder = 'Search...',
  onClear,
}: SearchBarProps) {
  return (
    <div className="relative flex-1">
      <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
      <Input
        type="text"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="pl-10 pr-10"
      />
      {value && onClear && (
        <button
          onClick={onClear}
          className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
        >
          <X className="h-4 w-4" />
        </button>
      )}
    </div>
  )
}

// ========================================================================
// STATUS FILTER COMPONENT
// ========================================================================

interface StatusFilterProps {
  value: string | string[]
  onChange: (value: string | string[]) => void
  options: { label: string; value: string }[]
  multiSelect?: boolean
}

export function StatusFilter({
  value,
  onChange,
  options,
  multiSelect = false,
}: StatusFilterProps) {
  const selectedCount = Array.isArray(value) ? value.length : value ? 1 : 0

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-2">
          <Filter className="h-4 w-4" />
          Status
          {selectedCount > 0 && (
            <span className="ml-1 rounded-full bg-blue-600 text-white text-xs px-2">
              {selectedCount}
            </span>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-48">
        <DropdownMenuLabel>Filter by status</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {multiSelect ? (
          <>
            {options.map((option) => (
              <DropdownMenuCheckboxItem
                key={option.value}
                checked={Array.isArray(value) && value.includes(option.value)}
                onCheckedChange={(checked) => {
                  const newValue = Array.isArray(value) ? [...value] : []
                  if (checked) {
                    newValue.push(option.value)
                  } else {
                    newValue.splice(newValue.indexOf(option.value), 1)
                  }
                  onChange(newValue)
                }}
              >
                {option.label}
              </DropdownMenuCheckboxItem>
            ))}
          </>
        ) : (
          <>
            {options.map((option) => (
              <DropdownMenuCheckboxItem
                key={option.value}
                checked={value === option.value}
                onCheckedChange={() => onChange(value === option.value ? '' : option.value)}
              >
                {option.label}
              </DropdownMenuCheckboxItem>
            ))}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ========================================================================
// DATE RANGE FILTER
// ========================================================================

interface DateRangeFilterProps {
  value: { start: string; end: string } | null
  onChange: (value: { start: string; end: string } | null) => void
}

export function DateRangeFilter({ value, onChange }: DateRangeFilterProps) {
  return (
    <div className="flex gap-2">
      <Input
        type="date"
        value={value?.start || ''}
        onChange={(e) =>
          onChange(
            e.target.value
              ? { start: e.target.value, end: value?.end || '' }
              : null
          )
        }
        placeholder="Start date"
      />
      <Input
        type="date"
        value={value?.end || ''}
        onChange={(e) =>
          onChange(
            e.target.value
              ? { start: value?.start || '', end: e.target.value }
              : null
          )
        }
        placeholder="End date"
      />
    </div>
  )
}

// ========================================================================
// SORT CONTROL
// ========================================================================

interface SortControlProps {
  column: string
  direction: 'asc' | 'desc'
  onChange: (column: string, direction: 'asc' | 'desc') => void
}

export function SortControl({ column, direction, onChange }: SortControlProps) {
  const toggleDirection = () => {
    onChange(column, direction === 'asc' ? 'desc' : 'asc')
  }

  return (
    <Button
      variant="outline"
      size="sm"
      onClick={toggleDirection}
      className="gap-2"
    >
      {direction === 'asc' ? '↑' : '↓'} Sort
    </Button>
  )
}

// ========================================================================
// FILTER BAR (COMBINED)
// ========================================================================

interface FilterBarProps {
  search?: string
  onSearchChange?: (value: string) => void
  filters?: {
    label: string
    value: string | string[]
    onChange: (value: string | string[]) => void
    options: { label: string; value: string }[]
  }[]
  onReset?: () => void
  searchPlaceholder?: string
}

export function FilterBar({
  search,
  onSearchChange,
  filters,
  onReset,
  searchPlaceholder = 'Search...',
}: FilterBarProps) {
  const activeFilterCount = filters?.reduce((count, filter) => {
    if (Array.isArray(filter.value)) {
      return count + (filter.value.length > 0 ? 1 : 0)
    }
    return count + (filter.value ? 1 : 0)
  }, 0) || 0

  const hasActiveSearch = (search || '').length > 0

  return (
    <div className="flex gap-2 items-center flex-wrap">
      {onSearchChange && (
        <SearchBar
          value={search || ''}
          onChange={onSearchChange}
          placeholder={searchPlaceholder}
          onClear={() => onSearchChange('')}
        />
      )}

      {filters?.map((filter, idx) => (
        <StatusFilter
          key={idx}
          value={filter.value}
          onChange={filter.onChange}
          options={filter.options}
        />
      ))}

      {(activeFilterCount > 0 || hasActiveSearch) && onReset && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onReset}
          className="text-gray-600"
        >
          Reset filters
        </Button>
      )}
    </div>
  )
}

// ========================================================================
// BULK ACTION TOOLBAR
// ========================================================================

interface BulkActionToolbarProps {
  selectedCount: number
  actions: {
    label: string
    onClick: () => void
    variant?: 'default' | 'destructive'
    confirm?: string
  }[]
  onSelectAll?: () => void
  onClearSelection?: () => void
}

export function BulkActionToolbar({
  selectedCount,
  actions,
  onClearSelection,
}: BulkActionToolbarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center gap-3 p-3 bg-blue-50 border border-blue-200 rounded-lg">
      <p className="text-sm font-medium text-blue-900">
        {selectedCount} selected
      </p>

      <div className="flex-1" />

      {onClearSelection && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClearSelection}
        >
          Clear
        </Button>
      )}

      {actions.map((action, idx) => (
        <Button
          key={idx}
          size="sm"
          variant={action.variant === 'destructive' ? 'destructive' : 'default'}
          onClick={() => {
            if (action.confirm) {
              if (window.confirm(action.confirm)) {
                action.onClick()
              }
            } else {
              action.onClick()
            }
          }}
        >
          {action.label}
        </Button>
      ))}
    </div>
  )
}

// ========================================================================
// COLUMN VISIBILITY FILTER
// ========================================================================

interface ColumnVisibilityFilterProps {
  columns: { key: string; label: string }[]
  visible: string[]
  onChange: (visible: string[]) => void
}

export function ColumnVisibilityFilter({
  columns,
  visible,
  onChange,
}: ColumnVisibilityFilterProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          Columns
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Show/Hide Columns</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {columns.map((column) => (
          <DropdownMenuCheckboxItem
            key={column.key}
            checked={visible.includes(column.key)}
            onCheckedChange={(checked) => {
              if (checked) {
                onChange([...visible, column.key])
              } else {
                onChange(visible.filter((k) => k !== column.key))
              }
            }}
          >
            {column.label}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
