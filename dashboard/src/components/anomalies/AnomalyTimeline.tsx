import { useMemo } from 'react'
import type { Anomaly, AnomalySeverity, AnomalyType } from '@/types/anomalies'

interface AnomalyTimelineProps {
  anomalies: Anomaly[]
  onSelect: (anomaly: Anomaly) => void
  selectedId?: string
}

const SEVERITY_COLORS: Record<AnomalySeverity, { bg: string; border: string; dot: string }> = {
  critical: { bg: 'bg-red-50', border: 'border-red-300', dot: 'bg-red-500' },
  warning: { bg: 'bg-yellow-50', border: 'border-yellow-300', dot: 'bg-yellow-500' },
}

const TYPE_LABELS: Record<AnomalyType, string> = {
  memory_leak: 'Memory Leak',
  cpu_spike: 'CPU Spike',
  oom_risk: 'OOM Risk',
}

export function AnomalyTimeline({ anomalies, onSelect, selectedId }: AnomalyTimelineProps) {
  const groupedByDate = useMemo(() => {
    const groups: Record<string, Anomaly[]> = {}
    anomalies.forEach((anomaly) => {
      const date = new Date(anomaly.detectedAt).toLocaleDateString()
      if (!groups[date]) groups[date] = []
      groups[date].push(anomaly)
    })
    return Object.entries(groups).sort(
      ([a], [b]) => new Date(b).getTime() - new Date(a).getTime()
    )
  }, [anomalies])

  const formatTime = (dateStr: string) => {
    return new Date(dateStr).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  if (anomalies.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center">
        <p className="text-gray-500">No anomalies found</p>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Anomaly Timeline</h2>
      <div className="space-y-6">
        {groupedByDate.map(([date, dateAnomalies]) => (
          <div key={date}>
            <div className="flex items-center gap-2 mb-3">
              <span className="text-sm font-medium text-gray-700">{date}</span>
              <span className="text-xs text-gray-400">
                ({dateAnomalies.length} anomal{dateAnomalies.length === 1 ? 'y' : 'ies'})
              </span>
            </div>
            <div className="relative pl-4 border-l-2 border-gray-200 space-y-3">
              {dateAnomalies.map((anomaly) => {
                const colors = SEVERITY_COLORS[anomaly.severity]
                const isSelected = anomaly.id === selectedId
                return (
                  <button
                    key={anomaly.id}
                    onClick={() => onSelect(anomaly)}
                    className={`w-full text-left relative pl-4 py-2 pr-3 rounded-lg border transition-all ${
                      isSelected
                        ? `${colors.bg} ${colors.border} ring-2 ring-offset-1 ring-blue-400`
                        : `${colors.bg} ${colors.border} hover:shadow-md`
                    }`}
                  >
                    <span
                      className={`absolute -left-[21px] top-1/2 -translate-y-1/2 w-3 h-3 rounded-full ${colors.dot} border-2 border-white`}
                    />
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-gray-900">
                            {TYPE_LABELS[anomaly.type]}
                          </span>
                          <span
                            className={`px-1.5 py-0.5 text-xs font-medium rounded ${
                              anomaly.severity === 'critical'
                                ? 'bg-red-100 text-red-700'
                                : 'bg-yellow-100 text-yellow-700'
                            }`}
                          >
                            {anomaly.severity}
                          </span>
                        </div>
                        <p className="text-sm text-gray-600 truncate">
                          {anomaly.namespace}/{anomaly.deployment}
                        </p>
                      </div>
                      <div className="text-right flex-shrink-0">
                        <span className="text-xs text-gray-500">{formatTime(anomaly.detectedAt)}</span>
                        {anomaly.status === 'resolved' && (
                          <span className="block text-xs text-green-600">Resolved</span>
                        )}
                      </div>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
