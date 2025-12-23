import type { AnomalyFilters as Filters, AnomalyType, AnomalySeverity } from '@/types/anomalies'

interface AnomalyFiltersProps {
  filters: Filters
  namespaces: string[]
  onFilterChange: (filters: Filters) => void
}

const ANOMALY_TYPES: { value: AnomalyType; label: string }[] = [
  { value: 'memory_leak', label: 'Memory Leak' },
  { value: 'cpu_spike', label: 'CPU Spike' },
  { value: 'oom_risk', label: 'OOM Risk' },
]

const SEVERITIES: { value: AnomalySeverity; label: string }[] = [
  { value: 'critical', label: 'Critical' },
  { value: 'warning', label: 'Warning' },
]

export function AnomalyFilters({ filters, namespaces, onFilterChange }: AnomalyFiltersProps) {
  const handleTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value as AnomalyType | ''
    onFilterChange({ ...filters, type: value || undefined })
  }

  const handleSeverityChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value as AnomalySeverity | ''
    onFilterChange({ ...filters, severity: value || undefined })
  }

  const handleNamespaceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value
    onFilterChange({ ...filters, namespace: value || undefined })
  }

  const handleStartDateChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value
    onFilterChange({ ...filters, startDate: value || undefined })
  }

  const handleEndDateChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value
    onFilterChange({ ...filters, endDate: value || undefined })
  }

  const handleClearFilters = () => {
    onFilterChange({})
  }

  const hasActiveFilters =
    filters.type || filters.severity || filters.namespace || filters.startDate || filters.endDate

  return (
    <div className="flex flex-wrap items-end gap-4 p-4 bg-white rounded-lg shadow-sm border border-gray-200">
      <div className="flex flex-col gap-1">
        <label htmlFor="type-filter" className="text-sm font-medium text-gray-700">
          Type
        </label>
        <select
          id="type-filter"
          value={filters.type || ''}
          onChange={handleTypeChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        >
          <option value="">All types</option>
          {ANOMALY_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="severity-filter" className="text-sm font-medium text-gray-700">
          Severity
        </label>
        <select
          id="severity-filter"
          value={filters.severity || ''}
          onChange={handleSeverityChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        >
          <option value="">All severities</option>
          {SEVERITIES.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="namespace-filter" className="text-sm font-medium text-gray-700">
          Namespace
        </label>
        <select
          id="namespace-filter"
          value={filters.namespace || ''}
          onChange={handleNamespaceChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        >
          <option value="">All namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="start-date" className="text-sm font-medium text-gray-700">
          From
        </label>
        <input
          type="date"
          id="start-date"
          value={filters.startDate || ''}
          onChange={handleStartDateChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="end-date" className="text-sm font-medium text-gray-700">
          To
        </label>
        <input
          type="date"
          id="end-date"
          value={filters.endDate || ''}
          onChange={handleEndDateChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        />
      </div>

      {hasActiveFilters && (
        <button
          onClick={handleClearFilters}
          className="text-sm text-blue-600 hover:text-blue-800 hover:underline pb-1.5"
        >
          Clear filters
        </button>
      )}
    </div>
  )
}
