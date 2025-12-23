import { useState, useEffect, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api'
import type { NamespaceCost } from '@/types/costs'

type SortField = 'namespace' | 'currentCost' | 'previousCost' | 'change' | 'containers'
type SortDirection = 'asc' | 'desc'

interface NamespaceCostTableProps {
  onNamespaceSelect?: (namespace: string) => void
}

export function NamespaceCostTable({ onNamespaceSelect }: NamespaceCostTableProps) {
  const navigate = useNavigate()
  const [costs, setCosts] = useState<NamespaceCost[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sortField, setSortField] = useState<SortField>('currentCost')
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc')

  const fetchNamespaceCosts = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getCosts('30d')
      const namespaceCosts: NamespaceCost[] = data.datasets.map((ds, idx) => ({
        namespace: ds.label,
        currentCost: ds.data[ds.data.length - 1] || 0,
        previousCost: ds.data[0] || 0,
        change:
          ds.data[0] > 0
            ? ((ds.data[ds.data.length - 1] - ds.data[0]) / ds.data[0]) * 100
            : 0,
        containers: 5 + idx * 3,
      }))
      setCosts(namespaceCosts)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load namespace costs')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchNamespaceCosts()
  }, [fetchNamespaceCosts])

  const sortedCosts = useMemo(() => {
    const sorted = [...costs]
    sorted.sort((a, b) => {
      let comparison = 0
      switch (sortField) {
        case 'namespace':
          comparison = a.namespace.localeCompare(b.namespace)
          break
        case 'currentCost':
          comparison = a.currentCost - b.currentCost
          break
        case 'previousCost':
          comparison = a.previousCost - b.previousCost
          break
        case 'change':
          comparison = a.change - b.change
          break
        case 'containers':
          comparison = a.containers - b.containers
          break
      }
      return sortDirection === 'asc' ? comparison : -comparison
    })
    return sorted
  }, [costs, sortField, sortDirection])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection('desc')
    }
  }

  const handleRowClick = (namespace: string) => {
    if (onNamespaceSelect) {
      onNamespaceSelect(namespace)
    } else {
      navigate(`/costs/namespace/${namespace}`)
    }
  }

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value)

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) return <span className="text-gray-300 ml-1">↕</span>
    return <span className="ml-1">{sortDirection === 'asc' ? '↑' : '↓'}</span>
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading namespace costs</p>
          <p className="text-sm mt-1">{error}</p>
          <button
            onClick={fetchNamespaceCosts}
            className="mt-3 text-sm text-red-600 hover:text-red-800 underline"
          >
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
      <div className="px-6 py-4 border-b border-gray-200">
        <h2 className="text-lg font-semibold text-gray-900">Namespace Costs</h2>
        <p className="text-sm text-gray-500 mt-1">
          Click a namespace to view detailed breakdown
        </p>
      </div>

      {loading ? (
        <div className="p-8 text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
          <p className="mt-2 text-gray-500">Loading namespace costs...</p>
        </div>
      ) : sortedCosts.length === 0 ? (
        <div className="p-8 text-center text-gray-500">
          <p>No namespace cost data available</p>
        </div>
      ) : (
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th
                onClick={() => handleSort('namespace')}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
              >
                Namespace <SortIcon field="namespace" />
              </th>
              <th
                onClick={() => handleSort('currentCost')}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
              >
                Current Cost <SortIcon field="currentCost" />
              </th>
              <th
                onClick={() => handleSort('previousCost')}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
              >
                Previous Cost <SortIcon field="previousCost" />
              </th>
              <th
                onClick={() => handleSort('change')}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
              >
                Change <SortIcon field="change" />
              </th>
              <th
                onClick={() => handleSort('containers')}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
              >
                Containers <SortIcon field="containers" />
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {sortedCosts.map((cost) => (
              <tr
                key={cost.namespace}
                onClick={() => handleRowClick(cost.namespace)}
                className="hover:bg-gray-50 cursor-pointer transition-colors"
              >
                <td className="px-6 py-4 whitespace-nowrap">
                  <span className="text-sm font-medium text-blue-600 hover:text-blue-800">
                    {cost.namespace}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                  {formatCurrency(cost.currentCost)}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {formatCurrency(cost.previousCost)}
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <span
                    className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      cost.change > 0
                        ? 'bg-red-100 text-red-800'
                        : cost.change < 0
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-100 text-gray-800'
                    }`}
                  >
                    {cost.change > 0 ? '+' : ''}
                    {cost.change.toFixed(1)}%
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {cost.containers}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
