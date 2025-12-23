import { type RecommendationFilters as Filters } from '@/types/recommendations'

interface RecommendationFiltersProps {
  filters: Filters
  namespaces: string[]
  onFilterChange: (filters: Filters) => void
}

export function RecommendationFilters({
  filters,
  namespaces,
  onFilterChange,
}: RecommendationFiltersProps) {
  const handleNamespaceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value
    onFilterChange({ ...filters, namespace: value || undefined })
  }

  const handleConfidenceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value
    onFilterChange({ ...filters, confidence: value ? Number(value) : undefined })
  }

  const handleStatusChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value as Filters['status'] | ''
    onFilterChange({ ...filters, status: value || undefined })
  }

  const handleClearFilters = () => {
    onFilterChange({})
  }

  const hasActiveFilters = filters.namespace || filters.confidence || filters.status

  return (
    <div className="flex flex-wrap items-center gap-4 p-4 bg-white rounded-lg shadow-sm border border-gray-200">
      <div className="flex items-center gap-2">
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

      <div className="flex items-center gap-2">
        <label htmlFor="confidence-filter" className="text-sm font-medium text-gray-700">
          Min Confidence
        </label>
        <select
          id="confidence-filter"
          value={filters.confidence || ''}
          onChange={handleConfidenceChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        >
          <option value="">Any</option>
          <option value="50">50%+</option>
          <option value="70">70%+</option>
          <option value="80">80%+</option>
          <option value="90">90%+</option>
        </select>
      </div>

      <div className="flex items-center gap-2">
        <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
          Status
        </label>
        <select
          id="status-filter"
          value={filters.status || ''}
          onChange={handleStatusChange}
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
        >
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="approved">Approved</option>
          <option value="dismissed">Dismissed</option>
          <option value="applied">Applied</option>
        </select>
      </div>

      {hasActiveFilters && (
        <button
          onClick={handleClearFilters}
          className="text-sm text-blue-600 hover:text-blue-800 hover:underline"
        >
          Clear filters
        </button>
      )}
    </div>
  )
}
