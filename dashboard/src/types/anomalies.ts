export type AnomalyType = 'memory_leak' | 'cpu_spike' | 'oom_risk'
export type AnomalySeverity = 'warning' | 'critical'

export interface Anomaly {
  id: string
  type: AnomalyType
  severity: AnomalySeverity
  namespace: string
  deployment: string
  container: string
  detectedAt: string
  resolvedAt?: string
  status: 'active' | 'resolved' | 'acknowledged'
}

export interface AnomalyFilters {
  type?: AnomalyType
  severity?: AnomalySeverity
  namespace?: string
  startDate?: string
  endDate?: string
}

export interface AnomalyDetail extends Anomaly {
  metrics: AnomalyMetric[]
  relatedRecommendations: string[]
}

export interface AnomalyMetric {
  timestamp: string
  value: number
  threshold: number
}
