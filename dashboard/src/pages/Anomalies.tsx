import { useState, useEffect, useCallback, useMemo } from 'react'
import { api } from '@/api'
import { AnomalyTimeline, AnomalyFilters, AnomalyDetail } from '@/components/anomalies'
import type { Anomaly, AnomalyFilters as Filters } from '@/types/anomalies'

export function Anomalies() {
  const [anomalies, setAnomalies] = useState<Anomaly[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filters, setFilters] = useState<Filters>({})
  const [selectedAnomaly, setSelectedAnomaly] = useState<Anomaly | null>(null)

  const fetchAnomalies = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getAnomalies(filters)
      setAnomalies(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load anomalies')
    } finally {
      setLoading(false)
    }
  }, [filters])

  useEffect(() => {
    fetchAnomalies()
  }, [fetchAnomalies])

  const namespaces = useMemo(() => {
    const ns = new Set(anomalies.map((a) => a.namespace))
    return Array.from(ns).sort()
  }, [anomalies])

  const handleFilterChange = (newFilters: Filters) => {
    setFilters(newFilters)
    setSelectedAnomaly(null)
  }

  const handleSelectAnomaly = (anomaly: Anomaly) => {
    setSelectedAnomaly(anomaly)
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading anomalies</p>
          <p className="text-sm mt-1">{error}</p>
          <button onClick={fetchAnomalies} className="mt-3 text-sm underline hover:text-red-800">
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900">Anomaly History</h1>
        <span className="text-sm text-gray-500">
          {anomalies.length} anomal{anomalies.length !== 1 ? 'ies' : 'y'}
        </span>
      </div>

      <AnomalyFilters filters={filters} namespaces={namespaces} onFilterChange={handleFilterChange} />

      {loading ? (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
          <p className="mt-2 text-gray-500">Loading anomalies...</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2">
            <AnomalyTimeline
              anomalies={anomalies}
              onSelect={handleSelectAnomaly}
              selectedId={selectedAnomaly?.id}
            />
          </div>
          <div>
            {selectedAnomaly ? (
              <AnomalyDetail anomaly={selectedAnomaly} />
            ) : (
              <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center text-gray-500">
                <p>Select an anomaly to view details</p>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
