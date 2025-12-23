import { useState, useEffect, useCallback } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  ReferenceDot,
} from 'recharts'
import { setCostData, setPeriod, setLoading } from '@/store/slices/costsSlice'
import { useAppDispatch, useAppSelector } from '@/store/hooks'
import { api } from '@/api'
import type { Period, CostDataset } from '@/types/costs'

interface RecommendationImpact {
  date: string
  label: string
  savings: number
}

interface CostChartProps {
  impacts?: RecommendationImpact[]
}

interface ChartDataPoint {
  date: string
  [key: string]: string | number
}

const PERIODS: { value: Period; label: string }[] = [
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
  { value: '90d', label: '90 Days' },
]

export function CostChart({ impacts = [] }: CostChartProps) {
  const dispatch = useAppDispatch()
  const data = useAppSelector((state) => state.costs.data)
  const period = useAppSelector((state) => state.costs.period)
  const loading = useAppSelector((state) => state.costs.loading)
  const [error, setError] = useState<string | null>(null)

  const fetchCosts = useCallback(async () => {
    dispatch(setLoading(true))
    setError(null)
    try {
      const costData = await api.getCosts(period)
      dispatch(setCostData(costData))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load cost data')
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch, period])

  useEffect(() => {
    fetchCosts()
  }, [fetchCosts])

  const handlePeriodChange = (newPeriod: Period) => {
    dispatch(setPeriod(newPeriod))
  }

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(value)

  const chartData: ChartDataPoint[] = data
    ? data.labels.map((label: string, idx: number) => ({
        date: label,
        ...data.datasets.reduce(
          (acc: Record<string, number>, ds: CostDataset) => {
            acc[ds.label] = ds.data[idx]
            return acc
          },
          {} as Record<string, number>
        ),
      }))
    : []

  const impactPoints = impacts.filter((impact) =>
    chartData.some((d: ChartDataPoint) => d.date === impact.date)
  )

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading cost data</p>
          <p className="text-sm mt-1">{error}</p>
          <button
            onClick={fetchCosts}
            className="mt-3 text-sm text-red-600 hover:text-red-800 underline"
          >
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Cost Trend</h2>
          {data && (
            <p className="text-sm text-gray-500 mt-1">
              Total: {formatCurrency(data.total)}
              <span
                className={`ml-2 ${data.change >= 0 ? 'text-red-600' : 'text-green-600'}`}
              >
                {data.change >= 0 ? '+' : ''}
                {data.change.toFixed(1)}%
              </span>
            </p>
          )}
        </div>
        <div className="flex gap-1 bg-gray-100 rounded-lg p-1">
          {PERIODS.map((p) => (
            <button
              key={p.value}
              onClick={() => handlePeriodChange(p.value)}
              className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
                period === p.value
                  ? 'bg-white text-gray-900 shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="h-80 flex items-center justify-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
        </div>
      ) : (
        <div className="h-80">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 12, fill: '#6b7280' }}
                tickLine={false}
                axisLine={{ stroke: '#e5e7eb' }}
              />
              <YAxis
                tickFormatter={(v) => `$${v}`}
                tick={{ fontSize: 12, fill: '#6b7280' }}
                tickLine={false}
                axisLine={{ stroke: '#e5e7eb' }}
              />
              <Tooltip
                formatter={(value) => [formatCurrency(Number(value)), '']}
                labelStyle={{ color: '#111827', fontWeight: 500 }}
                contentStyle={{
                  backgroundColor: 'white',
                  border: '1px solid #e5e7eb',
                  borderRadius: '8px',
                  boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1)',
                }}
              />
              <Legend />
              {data?.datasets.map((ds: CostDataset) => (
                <Line
                  key={ds.label}
                  type="monotone"
                  dataKey={ds.label}
                  stroke={ds.borderColor}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 6 }}
                />
              ))}
              {impactPoints.map((impact, idx) => {
                const dataPoint = chartData.find((d: ChartDataPoint) => d.date === impact.date)
                if (!dataPoint) return null
                const firstDataset = data?.datasets[0]?.label
                const yValue = firstDataset ? Number(dataPoint[firstDataset]) : 0
                return (
                  <ReferenceDot
                    key={idx}
                    x={impact.date}
                    y={yValue}
                    r={8}
                    fill="#10b981"
                    stroke="#fff"
                    strokeWidth={2}
                  />
                )
              })}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {impactPoints.length > 0 && (
        <div className="mt-4 pt-4 border-t border-gray-200">
          <h3 className="text-sm font-medium text-gray-700 mb-2">Recommendation Impacts</h3>
          <div className="flex flex-wrap gap-2">
            {impactPoints.map((impact, idx) => (
              <span
                key={idx}
                className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800"
              >
                <span className="w-2 h-2 rounded-full bg-green-500 mr-1.5" />
                {impact.date}: {impact.label} ({formatCurrency(impact.savings)} saved)
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
