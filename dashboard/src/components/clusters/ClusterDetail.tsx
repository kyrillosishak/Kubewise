import { useState, useEffect, useCallback } from 'react'
import { api } from '@/api'
import type { ClusterHealth, AgentStatus, ClusterStatus } from '@/types/clusters'

interface ClusterDetailProps {
  clusterId: string
  onBack?: () => void
}

const STATUS_CONFIG: Record<ClusterStatus, { color: string; bg: string; label: string }> = {
  healthy: { color: 'text-green-700', bg: 'bg-green-100', label: 'Healthy' },
  degraded: { color: 'text-yellow-700', bg: 'bg-yellow-100', label: 'Degraded' },
  disconnected: { color: 'text-red-700', bg: 'bg-red-100', label: 'Disconnected' },
}

const AGENT_STATUS_CONFIG: Record<AgentStatus['status'], { color: string; bg: string; dot: string }> = {
  running: { color: 'text-green-700', bg: 'bg-green-50', dot: 'bg-green-500' },
  stopped: { color: 'text-gray-600', bg: 'bg-gray-50', dot: 'bg-gray-400' },
  error: { color: 'text-red-700', bg: 'bg-red-50', dot: 'bg-red-500' },
}

export function ClusterDetail({ clusterId, onBack }: ClusterDetailProps) {
  const [health, setHealth] = useState<ClusterHealth | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchHealth = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getClusterHealth(clusterId)
      setHealth(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load cluster health')
    } finally {
      setLoading(false)
    }
  }, [clusterId])

  useEffect(() => {
    fetchHealth()
  }, [fetchHealth])

  const formatLastSeen = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleString()
  }

  const formatTimeSince = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffSecs = Math.floor(diffMs / 1000)
    
    if (diffSecs < 60) return `${diffSecs}s ago`
    const diffMins = Math.floor(diffSecs / 60)
    if (diffMins < 60) return `${diffMins}m ago`
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}h ago`
    return date.toLocaleDateString()
  }

  const getStatusBadge = (status: ClusterStatus) => {
    const config = STATUS_CONFIG[status]
    return (
      <span className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-full ${config.bg} ${config.color}`}>
        <span className={`w-2 h-2 rounded-full ${status === 'healthy' ? 'bg-green-500' : status === 'degraded' ? 'bg-yellow-500' : 'bg-red-500'}`} />
        {config.label}
      </span>
    )
  }

  const getUtilizationColor = (value: number) => {
    if (value >= 90) return 'bg-red-500'
    if (value >= 70) return 'bg-yellow-500'
    return 'bg-green-500'
  }

  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex justify-center py-12">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading cluster details</p>
          <p className="text-sm mt-1">{error}</p>
          <button onClick={fetchHealth} className="mt-3 text-sm underline hover:text-red-800">
            Try again
          </button>
        </div>
      </div>
    )
  }

  if (!health) return null

  const { cluster, agents, metrics } = health

  return (
    <div className="space-y-6">
      {onBack && (
        <button
          onClick={onBack}
          className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 transition-colors"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to clusters
        </button>
      )}

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex items-start justify-between mb-6">
          <div>
            <h2 className="text-xl font-semibold text-gray-900">{cluster.name}</h2>
            <p className="text-sm text-gray-500 mt-1">Last seen: {formatLastSeen(cluster.lastSeen)}</p>
          </div>
          {getStatusBadge(cluster.status)}
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-gray-50 rounded-lg p-4">
            <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">Containers</dt>
            <dd className="mt-1 text-2xl font-semibold text-gray-900">{cluster.containersMonitored.toLocaleString()}</dd>
          </div>
          <div className="bg-gray-50 rounded-lg p-4">
            <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">Predictions</dt>
            <dd className="mt-1 text-2xl font-semibold text-gray-900">{cluster.predictionsGenerated.toLocaleString()}</dd>
          </div>
          <div className="bg-gray-50 rounded-lg p-4">
            <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">Anomalies</dt>
            <dd className="mt-1 text-2xl font-semibold text-gray-900">{cluster.anomaliesDetected.toLocaleString()}</dd>
          </div>
          <div className="bg-gray-50 rounded-lg p-4">
            <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">Model Version</dt>
            <dd className="mt-1 text-lg font-mono text-gray-900">{cluster.modelVersion}</dd>
          </div>
        </div>

        <div className="border-t border-gray-200 pt-6">
          <h3 className="text-sm font-medium text-gray-700 mb-4">Cluster Metrics</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-gray-600">CPU</span>
                <span className="text-sm font-medium text-gray-900">{metrics.cpuUtilization}%</span>
              </div>
              <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                <div className={`h-full ${getUtilizationColor(metrics.cpuUtilization)} transition-all`} style={{ width: `${metrics.cpuUtilization}%` }} />
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-gray-600">Memory</span>
                <span className="text-sm font-medium text-gray-900">{metrics.memoryUtilization}%</span>
              </div>
              <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                <div className={`h-full ${getUtilizationColor(metrics.memoryUtilization)} transition-all`} style={{ width: `${metrics.memoryUtilization}%` }} />
              </div>
            </div>
            <div className="bg-gray-50 rounded-lg p-3 text-center">
              <dt className="text-xs text-gray-500">Pods</dt>
              <dd className="text-lg font-semibold text-gray-900">{metrics.podCount}</dd>
            </div>
            <div className="bg-gray-50 rounded-lg p-3 text-center">
              <dt className="text-xs text-gray-500">Nodes</dt>
              <dd className="text-lg font-semibold text-gray-900">{metrics.nodeCount}</dd>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-4">Agent Status</h3>
        
        {agents.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <p>No agents found for this cluster</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead>
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Node</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Version</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Heartbeat</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {agents.map((agent) => {
                  const statusConfig = AGENT_STATUS_CONFIG[agent.status]
                  return (
                    <tr key={agent.nodeId} className={`${statusConfig.bg}`}>
                      <td className="px-4 py-3">
                        <div>
                          <p className="text-sm font-medium text-gray-900">{agent.nodeName}</p>
                          <p className="text-xs text-gray-500 font-mono">{agent.nodeId.slice(0, 12)}...</p>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex items-center gap-1.5 text-sm ${statusConfig.color}`}>
                          <span className={`w-2 h-2 rounded-full ${statusConfig.dot}`} />
                          {agent.status.charAt(0).toUpperCase() + agent.status.slice(1)}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <span className="text-sm font-mono text-gray-700">{agent.version}</span>
                      </td>
                      <td className="px-4 py-3">
                        <span className="text-sm text-gray-600">{formatTimeSince(agent.lastHeartbeat)}</span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
