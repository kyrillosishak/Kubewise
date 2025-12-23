import { useState, useEffect, useCallback } from 'react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { api } from '@/api'
import type { SavingsData } from '@/types/costs'

interface SavingsSummaryProps {
  since?: string
}

export function SavingsSummary({ since = '30d' }: SavingsSummaryProps) {
  const [data, setData] = useState<SavingsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchSavings = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const savingsData = await api.getSavings(since)
      setData(savingsData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load savings data')
    } finally {
      setLoading(false)
    }
  }, [since])

  useEffect(() => {
    fetchSavings()
  }, [fetchSavings])

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(value)

  const formatCompactCurrency = (value: number) => {
    if (value >= 1000) {
      return `$${(value / 1000).toFixed(1)}k`
    }
    return `$${value}`
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading savings data</p>
          <p className="text-sm mt-1">{error}</p>
          <button
            onClick={fetchSavings}
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
      <h2 className="text-lg font-semibold text-gray-900 mb-6">Savings Summary</h2>

      {loading ? (
        <div className="h-64 flex items-center justify-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
        </div>
      ) : data ? (
        <>
          <div className="grid grid-cols-2 gap-4 mb-6">
            <div className="bg-green-50 rounded-lg p-4">
              <p className="text-sm font-medium text-green-700">Realized Savings</p>
              <p className="text-2xl font-bold text-green-900 mt-1">
                {formatCurrency(data.realized)}
              </p>
              <p className="text-xs text-green-600 mt-1">
                From applied recommendations
              </p>
            </div>
            <div className="bg-blue-50 rounded-lg p-4">
              <p className="text-sm font-medium text-blue-700">Projected Savings</p>
              <p className="text-2xl font-bold text-blue-900 mt-1">
                {formatCurrency(data.projected)}
              </p>
              <p className="text-xs text-blue-600 mt-1">
                From pending recommendations
              </p>
            </div>
          </div>

          <div className="mb-4">
            <h3 className="text-sm font-medium text-gray-700 mb-2">Savings Trend</h3>
            <div className="h-48">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart
                  data={data.trend}
                  margin={{ top: 5, right: 10, left: 0, bottom: 5 }}
                >
                  <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                  <XAxis
                    dataKey="date"
                    tick={{ fontSize: 11, fill: '#6b7280' }}
                    tickLine={false}
                    axisLine={{ stroke: '#e5e7eb' }}
                  />
                  <YAxis
                    tickFormatter={formatCompactCurrency}
                    tick={{ fontSize: 11, fill: '#6b7280' }}
                    tickLine={false}
                    axisLine={{ stroke: '#e5e7eb' }}
                    width={50}
                  />
                  <Tooltip
                    formatter={(value, name) => [
                      formatCurrency(Number(value)),
                      name === 'realized' ? 'Realized' : 'Projected',
                    ]}
                    labelStyle={{ color: '#111827', fontWeight: 500 }}
                    contentStyle={{
                      backgroundColor: 'white',
                      border: '1px solid #e5e7eb',
                      borderRadius: '8px',
                      boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1)',
                    }}
                  />
                  <Legend
                    formatter={(value) =>
                      value === 'realized' ? 'Realized' : 'Projected'
                    }
                  />
                  <Area
                    type="monotone"
                    dataKey="realized"
                    stackId="1"
                    stroke="#10b981"
                    fill="#d1fae5"
                    strokeWidth={2}
                  />
                  <Area
                    type="monotone"
                    dataKey="projected"
                    stackId="2"
                    stroke="#3b82f6"
                    fill="#dbeafe"
                    strokeWidth={2}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="pt-4 border-t border-gray-200">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-500">Total Potential Savings</span>
              <span className="font-semibold text-gray-900">
                {formatCurrency(data.realized + data.projected)}
              </span>
            </div>
            <div className="mt-2">
              <div className="flex h-2 rounded-full overflow-hidden bg-gray-200">
                <div
                  className="bg-green-500"
                  style={{
                    width: `${(data.realized / (data.realized + data.projected)) * 100}%`,
                  }}
                />
                <div
                  className="bg-blue-500"
                  style={{
                    width: `${(data.projected / (data.realized + data.projected)) * 100}%`,
                  }}
                />
              </div>
              <div className="flex justify-between mt-1 text-xs text-gray-500">
                <span>
                  {((data.realized / (data.realized + data.projected)) * 100).toFixed(0)}%
                  realized
                </span>
                <span>
                  {((data.projected / (data.realized + data.projected)) * 100).toFixed(0)}%
                  projected
                </span>
              </div>
            </div>
          </div>
        </>
      ) : (
        <div className="text-center text-gray-500 py-8">
          <p>No savings data available</p>
        </div>
      )}
    </div>
  )
}
