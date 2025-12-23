import { useState, useEffect, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useDispatch, useSelector } from 'react-redux'
import { setRecommendations, setLoading, setFilters } from '@/store/slices/recommendationsSlice'
import { api } from '@/api'
import { RecommendationFilters } from './RecommendationFilters'
import type { Recommendation, RecommendationFilters as Filters } from '@/types/recommendations'
import type { RootState, AppDispatch } from '@/store'

type SortField = 'estimatedSavings' | 'confidence' | 'deployment' | 'namespace' | 'createdAt'
type SortDirection = 'asc' | 'desc'

export function RecommendationList() {
  const dispatch = useDispatch<AppDispatch>()
  const navigate = useNavigate()
  const items = useSelector((state: RootState) => state.recommendations.items)
  const loading = useSelector((state: RootState) => state.recommendations.loading)
  const filters = useSelector((state: RootState) => state.recommendations.filters)
  const [sortField, setSortField] = useState<SortField>('estimatedSavings')
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc')
  const [error, setError] = useState<string | null>(null)

  const fetchRecommendations = useCallback(async () => {
    dispatch(setLoading(true))
    setError(null)
    try {
      const data = await api.getRecommendations(filters)
      dispatch(setRecommendations(data))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load recommendations')
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch, filters])

  useEffect(() => {
    fetchRecommendations()
  }, [fetchRecommendations])

  const namespaces = useMemo((): string[] => {
    const ns = new Set(items.map((r: Recommendation) => r.namespace))
    return Array.from(ns).sort()
  }, [items])

  const filteredAndSortedItems = useMemo(() => {
    let result = [...items]

    if (filters.namespace) {
      result = result.filter((r) => r.namespace === filters.namespace)
    }
    if (filters.confidence) {
      result = result.filter((r) => r.confidence >= filters.confidence!)
    }
    if (filters.status) {
      result = result.filter((r) => r.status === filters.status)
    }

    result.sort((a, b) => {
      let comparison = 0
      switch (sortField) {
        case 'estimatedSavings':
          comparison = a.estimatedSavings - b.estimatedSavings
          break
        case 'confidence':
          comparison = a.confidence - b.confidence
          break
        case 'deployment':
          comparison = a.deployment.localeCompare(b.deployment)
          break
        case 'namespace':
          comparison = a.namespace.localeCompare(b.namespace)
          break
        case 'createdAt':
          comparison = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()
          break
      }
      return sortDirection === 'asc' ? comparison : -comparison
    })

    return result
  }, [items, filters, sortField, sortDirection])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection('desc')
    }
  }

  const handleFilterChange = (newFilters: Filters) => {
    dispatch(setFilters(newFilters))
  }

  const handleRowClick = (id: string) => {
    navigate(`/recommendations/${id}`)
  }

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value)
  }

  const formatConfidence = (value: number) => `${Math.round(value * 100)}%`

  const getStatusBadge = (status: Recommendation['status']) => {
    const styles: Record<Recommendation['status'], string> = {
      pending: 'bg-yellow-100 text-yellow-800',
      approved: 'bg-blue-100 text-blue-800',
      dismissed: 'bg-gray-100 text-gray-800',
      applied: 'bg-green-100 text-green-800',
    }
    return (
      <span className={`px-2 py-1 text-xs font-medium rounded-full ${styles[status]}`}>
        {status}
      </span>
    )
  }

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) return <span className="text-gray-300 ml-1">↕</span>
    return <span className="ml-1">{sortDirection === 'asc' ? '↑' : '↓'}</span>
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading recommendations</p>
          <p className="text-sm mt-1">{error}</p>
          <button
            onClick={fetchRecommendations}
            className="mt-3 text-sm text-red-600 hover:text-red-800 underline"
          >
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900">Recommendations</h1>
        <span className="text-sm text-gray-500">
          {filteredAndSortedItems.length} recommendation{filteredAndSortedItems.length !== 1 && 's'}
        </span>
      </div>

      <RecommendationFilters
        filters={filters}
        namespaces={namespaces}
        onFilterChange={handleFilterChange}
      />

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
        {loading ? (
          <div className="p-8 text-center">
            <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
            <p className="mt-2 text-gray-500">Loading recommendations...</p>
          </div>
        ) : filteredAndSortedItems.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            <p>No recommendations found</p>
            {Object.keys(filters).length > 0 && (
              <button
                onClick={() => handleFilterChange({})}
                className="mt-2 text-blue-600 hover:underline"
              >
                Clear filters
              </button>
            )}
          </div>
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th
                  onClick={() => handleSort('deployment')}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                >
                  Deployment <SortIcon field="deployment" />
                </th>
                <th
                  onClick={() => handleSort('namespace')}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                >
                  Namespace <SortIcon field="namespace" />
                </th>
                <th
                  onClick={() => handleSort('confidence')}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                >
                  Confidence <SortIcon field="confidence" />
                </th>
                <th
                  onClick={() => handleSort('estimatedSavings')}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                >
                  Est. Savings <SortIcon field="estimatedSavings" />
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th
                  onClick={() => handleSort('createdAt')}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                >
                  Created <SortIcon field="createdAt" />
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {filteredAndSortedItems.map((rec) => (
                <tr
                  key={rec.id}
                  onClick={() => handleRowClick(rec.id)}
                  className="hover:bg-gray-50 cursor-pointer transition-colors"
                >
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900">{rec.deployment}</div>
                    <div className="text-sm text-gray-500">{rec.container}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {rec.namespace}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="w-16 bg-gray-200 rounded-full h-2 mr-2">
                        <div
                          className="bg-blue-600 h-2 rounded-full"
                          style={{ width: `${rec.confidence * 100}%` }}
                        />
                      </div>
                      <span className="text-sm text-gray-700">{formatConfidence(rec.confidence)}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-green-600">
                    {formatCurrency(rec.estimatedSavings)}/mo
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">{getStatusBadge(rec.status)}</td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(rec.createdAt).toLocaleDateString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
