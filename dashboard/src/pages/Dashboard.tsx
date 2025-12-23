import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '@/api'
import type { Recommendation } from '@/types/recommendations'
import type { Cluster } from '@/types/clusters'
import type { Anomaly } from '@/types/anomalies'

interface DashboardMetrics {
  totalSavings: number
  pendingRecommendations: number
  activeAnomalies: number
  healthyClusters: number
  totalClusters: number
}

export function Dashboard() {
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
  const [recentRecommendations, setRecentRecommendations] = useState<Recommendation[]>([])
  const [recentAnomalies, setRecentAnomalies] = useState<Anomaly[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchDashboardData = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [recommendations, clusters, anomalies] = await Promise.all([
        api.getRecommendations({ status: 'pending' }),
        api.getClusters(),
        api.getAnomalies({}),
      ])

      const pendingRecs = recommendations.filter((r: Recommendation) => r.status === 'pending')
      const totalSavings = pendingRecs.reduce((sum: number, r: Recommendation) => sum + r.estimatedSavings, 0)
      const healthyClusters = clusters.filter((c: Cluster) => c.status === 'healthy').length
      const activeAnomalies = anomalies.filter((a: Anomaly) => a.status === 'active').length

      setMetrics({
        totalSavings,
        pendingRecommendations: pendingRecs.length,
        activeAnomalies,
        healthyClusters,
        totalClusters: clusters.length,
      })

      setRecentRecommendations(pendingRecs.slice(0, 5))
      setRecentAnomalies(anomalies.slice(0, 5))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchDashboardData()
  }, [fetchDashboardData])

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(value)
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading dashboard</p>
          <p className="text-sm mt-1">{error}</p>
          <button onClick={fetchDashboardData} className="mt-3 text-sm underline hover:text-red-800">
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900">Dashboard</h1>
        <span className="text-sm text-gray-500">
          Last updated: {new Date().toLocaleTimeString()}
        </span>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 animate-pulse">
              <div className="h-4 bg-gray-200 rounded w-1/2 mb-4" />
              <div className="h-8 bg-gray-200 rounded w-3/4" />
            </div>
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <MetricCard
              title="Potential Savings"
              value={formatCurrency(metrics?.totalSavings || 0)}
              subtitle="/month"
              icon={
                <svg className="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              }
              color="green"
              link="/costs"
            />
            <MetricCard
              title="Pending Recommendations"
              value={String(metrics?.pendingRecommendations || 0)}
              subtitle="awaiting review"
              icon={
                <svg className="w-6 h-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                </svg>
              }
              color="blue"
              link="/recommendations"
            />
            <MetricCard
              title="Active Anomalies"
              value={String(metrics?.activeAnomalies || 0)}
              subtitle="need attention"
              icon={
                <svg className="w-6 h-6 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              }
              color="yellow"
              link="/anomalies"
            />
            <MetricCard
              title="Cluster Health"
              value={`${metrics?.healthyClusters || 0}/${metrics?.totalClusters || 0}`}
              subtitle="clusters healthy"
              icon={
                <svg className="w-6 h-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              }
              color="purple"
              link="/clusters"
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <RecentRecommendations recommendations={recentRecommendations} />
            <RecentAnomalies anomalies={recentAnomalies} />
          </div>

          <QuickLinks />
        </>
      )}
    </div>
  )
}

interface MetricCardProps {
  title: string
  value: string
  subtitle: string
  icon: React.ReactNode
  color: 'green' | 'blue' | 'yellow' | 'purple'
  link: string
}

function MetricCard({ title, value, subtitle, icon, color, link }: MetricCardProps) {
  const bgColors = {
    green: 'bg-green-50',
    blue: 'bg-blue-50',
    yellow: 'bg-yellow-50',
    purple: 'bg-purple-50',
  }

  return (
    <Link
      to={link}
      className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 hover:shadow-md hover:border-gray-300 transition-all"
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-medium text-gray-500">{title}</p>
          <p className="mt-2 text-3xl font-semibold text-gray-900">{value}</p>
          <p className="mt-1 text-sm text-gray-500">{subtitle}</p>
        </div>
        <div className={`p-3 rounded-lg ${bgColors[color]}`}>{icon}</div>
      </div>
    </Link>
  )
}

function RecentRecommendations({ recommendations }: { recommendations: Recommendation[] }) {
  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200">
      <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Recent Recommendations</h2>
        <Link to="/recommendations" className="text-sm text-blue-600 hover:text-blue-800">
          View all →
        </Link>
      </div>
      {recommendations.length === 0 ? (
        <div className="p-6 text-center text-gray-500">No pending recommendations</div>
      ) : (
        <ul className="divide-y divide-gray-200">
          {recommendations.map((rec) => (
            <li key={rec.id}>
              <Link
                to={`/recommendations/${rec.id}`}
                className="block px-6 py-4 hover:bg-gray-50 transition-colors"
              >
                <div className="flex items-center justify-between">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900 truncate">{rec.deployment}</p>
                    <p className="text-sm text-gray-500">{rec.namespace}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-medium text-green-600">
                      ${rec.estimatedSavings.toFixed(0)}/mo
                    </p>
                    <p className="text-xs text-gray-500">{Math.round(rec.confidence * 100)}% confidence</p>
                  </div>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function RecentAnomalies({ anomalies }: { anomalies: Anomaly[] }) {
  const typeLabels: Record<string, string> = {
    memory_leak: 'Memory Leak',
    cpu_spike: 'CPU Spike',
    oom_risk: 'OOM Risk',
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200">
      <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Recent Anomalies</h2>
        <Link to="/anomalies" className="text-sm text-blue-600 hover:text-blue-800">
          View all →
        </Link>
      </div>
      {anomalies.length === 0 ? (
        <div className="p-6 text-center text-gray-500">No recent anomalies</div>
      ) : (
        <ul className="divide-y divide-gray-200">
          {anomalies.map((anomaly) => (
            <li key={anomaly.id} className="px-6 py-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-gray-900">{typeLabels[anomaly.type] || anomaly.type}</p>
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
                  <p className="text-sm text-gray-500">
                    {anomaly.namespace}/{anomaly.deployment}
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-xs text-gray-500">
                    {new Date(anomaly.detectedAt).toLocaleDateString()}
                  </p>
                  <p
                    className={`text-xs ${
                      anomaly.status === 'resolved' ? 'text-green-600' : 'text-yellow-600'
                    }`}
                  >
                    {anomaly.status}
                  </p>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function QuickLinks() {
  const links = [
    { to: '/recommendations', label: 'Review Recommendations', description: 'Optimize resource allocation' },
    { to: '/costs', label: 'Cost Analytics', description: 'Track savings and spending' },
    { to: '/anomalies', label: 'Anomaly History', description: 'Review detected issues' },
    { to: '/clusters', label: 'Cluster Health', description: 'Monitor cluster status' },
  ]

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Quick Links</h2>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {links.map((link) => (
          <Link
            key={link.to}
            to={link.to}
            className="p-4 rounded-lg border border-gray-200 hover:border-blue-300 hover:bg-blue-50 transition-all text-center"
          >
            <p className="text-sm font-medium text-gray-900">{link.label}</p>
            <p className="text-xs text-gray-500 mt-1">{link.description}</p>
          </Link>
        ))}
      </div>
    </div>
  )
}
