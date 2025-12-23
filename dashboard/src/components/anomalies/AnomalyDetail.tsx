import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '@/api'
import type { Anomaly, AnomalyDetail as AnomalyDetailType, AnomalyType } from '@/types/anomalies'

interface AnomalyDetailProps {
  anomaly: Anomaly
}

const TYPE_LABELS: Record<AnomalyType, string> = {
  memory_leak: 'Memory Leak',
  cpu_spike: 'CPU Spike',
  oom_risk: 'OOM Risk',
}

export function AnomalyDetail({ anomaly }: AnomalyDetailProps) {
  const [detail, setDetail] = useState<AnomalyDetailType | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchDetail = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getAnomalyDetail(anomaly.id)
      setDetail(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load anomaly details')
    } finally {
      setLoading(false)
    }
  }, [anomaly.id])

  useEffect(() => {
    fetchDetail()
  }, [fetchDetail])

  const formatDateTime = (dateStr: string) => new Date(dateStr).toLocaleString()

  const getStatusBadge = (status: Anomaly['status']) => {
    const styles: Record<Anomaly['status'], string> = {
      active: 'bg-red-100 text-red-800',
      acknowledged: 'bg-yellow-100 text-yellow-800',
      resolved: 'bg-green-100 text-green-800',
    }
    return (
      <span className={`px-2.5 py-1 text-sm font-medium rounded-full ${styles[status]}`}>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    )
  }

  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex justify-center py-8">
          <div className="inline-block animate-spin rounded-full h-6 w-6 border-4 border-blue-500 border-t-transparent" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading details</p>
          <p className="text-sm mt-1">{error}</p>
          <button onClick={fetchDetail} className="mt-2 text-sm underline hover:text-red-800">
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">{TYPE_LABELS[anomaly.type]}</h2>
          <p className="text-sm text-gray-500 mt-1">
            {anomaly.namespace}/{anomaly.deployment}/{anomaly.container}
          </p>
        </div>
        {getStatusBadge(anomaly.status)}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="bg-gray-50 rounded-lg p-4">
          <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">Detected</dt>
          <dd className="mt-1 text-sm font-medium text-gray-900">
            {formatDateTime(anomaly.detectedAt)}
          </dd>
        </div>
        <div className="bg-gray-50 rounded-lg p-4">
          <dt className="text-xs font-medium text-gray-500 uppercase tracking-wide">
            {anomaly.resolvedAt ? 'Resolved' : 'Duration'}
          </dt>
          <dd className="mt-1 text-sm font-medium text-gray-900">
            {anomaly.resolvedAt ? formatDateTime(anomaly.resolvedAt) : 'Ongoing'}
          </dd>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium text-gray-700 mb-2">Container Details</h3>
        <dl className="bg-gray-50 rounded-lg p-4 space-y-2">
          <div className="flex justify-between">
            <dt className="text-sm text-gray-500">Namespace</dt>
            <dd className="text-sm font-medium text-gray-900">{anomaly.namespace}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-sm text-gray-500">Deployment</dt>
            <dd className="text-sm font-medium text-gray-900">{anomaly.deployment}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-sm text-gray-500">Container</dt>
            <dd className="text-sm font-medium text-gray-900">{anomaly.container}</dd>
          </div>
        </dl>
      </div>

      {detail?.relatedRecommendations && detail.relatedRecommendations.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-gray-700 mb-2">Related Recommendations</h3>
          <ul className="space-y-2">
            {detail.relatedRecommendations.map((recId) => (
              <li key={recId}>
                <Link
                  to={`/recommendations/${recId}`}
                  className="flex items-center gap-2 p-3 bg-blue-50 rounded-lg text-blue-700 hover:bg-blue-100 transition-colors"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M13 7l5 5m0 0l-5 5m5-5H6"
                    />
                  </svg>
                  <span className="text-sm font-medium">View recommendation {recId.slice(0, 8)}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
