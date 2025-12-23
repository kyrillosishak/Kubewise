import { useState, useEffect, useCallback } from 'react'
import { api } from '@/api'
import type { Cluster, ClusterStatus } from '@/types/clusters'

interface ClusterHealthGridProps {
  onSelectCluster?: (clusterId: string) => void
}

const STATUS_CONFIG: Record<ClusterStatus, { color: string; bg: string; label: string }> = {
  healthy: { color: 'text-green-700', bg: 'bg-green-100', label: 'Healthy' },
  degraded: { color: 'text-yellow-700', bg: 'bg-yellow-100', label: 'Degraded' },
  disconnected: { color: 'text-red-700', bg: 'bg-red-100', label: 'Disconnected' },
}

export function ClusterHealthGrid({ onSelectCluster }: ClusterHealthGridProps) {
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchClusters = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getClusters()
      setClusters(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load clusters')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchClusters()
  }, [fetchClusters])

  const getStatusIndicator = (status: ClusterStatus) => {
    const config = STATUS_CONFIG[status]
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-full ${config.bg} ${config.color}`}>
        <span className={`w-2 h-2 rounded-full ${status === 'healthy' ? 'bg-green-500' : status === 'degraded' ? 'bg-yellow-500' : 'bg-red-500'}`} />
        {config.label}
      </span>
    )
  }

  const formatLastSeen = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    
    if (diffMins < 1) return 'Just now'
    if (diffMins < 60) return `${diffMins}m ago`
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}h ago`
    return date.toLocaleDateString()
  }

  if (loading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {[1, 2, 3].map((i) => (
          <div key={i} className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 animate-pulse">
            <div className="h-4 bg-gray-200 rounded w-1/2 mb-4" />
            <div className="h-3 bg-gray-200 rounded w-1/4 mb-6" />
            <div className="space-y-3">
              <div className="h-3 bg-gray-200 rounded w-3/4" />
              <div className="h-3 bg-gray-200 rounded w-2/3" />
              <div className="h-3 bg-gray-200 rounded w-1/2" />
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-red-700">
        <p className="font-medium">Error loading clusters</p>
        <p className="text-sm mt-1">{error}</p>
        <button onClick={fetchClusters} className="mt-3 text-sm underline hover:text-red-800">
          Try again
        </button>
      </div>
    )
  }

  if (clusters.length === 0) {
    return (
      <div className="bg-gray-50 border border-gray-200 rounded-lg p-8 text-center">
        <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
        <p className="mt-4 text-gray-600 font-medium">No clusters connected</p>
        <p className="mt-1 text-sm text-gray-500">Connect a cluster to start monitoring</p>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {clusters.map((cluster) => (
        <div
          key={cluster.id}
          onClick={() => onSelectCluster?.(cluster.id)}
          className={`bg-white rounded-lg shadow-sm border border-gray-200 p-6 transition-all ${
            onSelectCluster ? 'cursor-pointer hover:shadow-md hover:border-blue-300' : ''
          } ${cluster.status !== 'healthy' ? 'border-l-4 border-l-yellow-400' : ''}`}
        >
          <div className="flex items-start justify-between mb-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900">{cluster.name}</h3>
              <p className="text-xs text-gray-500 mt-0.5">Last seen: {formatLastSeen(cluster.lastSeen)}</p>
            </div>
            {getStatusIndicator(cluster.status)}
          </div>

          <dl className="space-y-3">
            <div className="flex items-center justify-between">
              <dt className="text-sm text-gray-500 flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                </svg>
                Containers
              </dt>
              <dd className="text-sm font-medium text-gray-900">{cluster.containersMonitored.toLocaleString()}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-sm text-gray-500 flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                </svg>
                Predictions
              </dt>
              <dd className="text-sm font-medium text-gray-900">{cluster.predictionsGenerated.toLocaleString()}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-sm text-gray-500 flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                Anomalies
              </dt>
              <dd className="text-sm font-medium text-gray-900">{cluster.anomaliesDetected.toLocaleString()}</dd>
            </div>
          </dl>

          <div className="mt-4 pt-4 border-t border-gray-100">
            <div className="flex items-center justify-between text-xs">
              <span className="text-gray-500">Model Version</span>
              <span className="font-mono text-gray-700 bg-gray-100 px-2 py-0.5 rounded">{cluster.modelVersion}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
