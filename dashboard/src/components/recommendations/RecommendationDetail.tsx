import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useDispatch } from 'react-redux'
import { api } from '@/api'
import { updateRecommendation } from '@/store/slices/recommendationsSlice'
import { ResourceComparison } from './ResourceComparison'
import type { RecommendationDetail as RecommendationDetailType } from '@/types/recommendations'
import type { AppDispatch } from '@/store'

export function RecommendationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const dispatch = useDispatch<AppDispatch>()
  const [recommendation, setRecommendation] = useState<RecommendationDetailType | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<'approve' | 'dismiss' | null>(null)

  const fetchRecommendation = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setError(null)
    try {
      const data = await api.getRecommendation(id)
      setRecommendation(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load recommendation')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchRecommendation()
  }, [fetchRecommendation])

  const handleApprove = async () => {
    if (!recommendation) return
    setActionLoading('approve')
    try {
      await api.approveRecommendation(recommendation.id, {
        approver: 'current-user',
        reason: 'Approved via dashboard',
      })
      const updated = { ...recommendation, status: 'approved' as const }
      setRecommendation(updated)
      dispatch(updateRecommendation(updated))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve recommendation')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDismiss = async () => {
    if (!recommendation) return
    setActionLoading('dismiss')
    try {
      await api.dismissRecommendation(recommendation.id)
      const updated = { ...recommendation, status: 'dismissed' as const }
      setRecommendation(updated)
      dispatch(updateRecommendation(updated))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to dismiss recommendation')
    } finally {
      setActionLoading(null)
    }
  }

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value)

  const formatConfidence = (value: number) => `${Math.round(value * 100)}%`

  const getStatusBadge = (status: RecommendationDetailType['status']) => {
    const styles: Record<RecommendationDetailType['status'], string> = {
      pending: 'bg-yellow-100 text-yellow-800',
      approved: 'bg-blue-100 text-blue-800',
      dismissed: 'bg-gray-100 text-gray-800',
      applied: 'bg-green-100 text-green-800',
    }
    return (
      <span className={`px-3 py-1 text-sm font-medium rounded-full ${styles[status]}`}>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    )
  }

  if (loading) {
    return (
      <div className="p-8 flex justify-center">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent" />
          <p className="mt-2 text-gray-500">Loading recommendation...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Error loading recommendation</p>
          <p className="text-sm mt-1">{error}</p>
          <button
            onClick={fetchRecommendation}
            className="mt-3 text-sm text-red-600 hover:text-red-800 underline"
          >
            Try again
          </button>
        </div>
      </div>
    )
  }

  if (!recommendation) {
    return (
      <div className="p-8 text-center text-gray-500">
        <p>Recommendation not found</p>
        <button
          onClick={() => navigate('/recommendations')}
          className="mt-2 text-blue-600 hover:underline"
        >
          Back to recommendations
        </button>
      </div>
    )
  }

  const canTakeAction = recommendation.status === 'pending'

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate('/recommendations')}
            className="text-gray-500 hover:text-gray-700"
          >
            ← Back
          </button>
          <h1 className="text-2xl font-semibold text-gray-900">Recommendation Details</h1>
        </div>
        {getStatusBadge(recommendation.status)}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Deployment Info</h2>
          <dl className="space-y-3">
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Deployment</dt>
              <dd className="text-sm font-medium text-gray-900">{recommendation.deployment}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Namespace</dt>
              <dd className="text-sm font-medium text-gray-900">{recommendation.namespace}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Container</dt>
              <dd className="text-sm font-medium text-gray-900">{recommendation.container}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Created</dt>
              <dd className="text-sm font-medium text-gray-900">
                {new Date(recommendation.createdAt).toLocaleString()}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Last Updated</dt>
              <dd className="text-sm font-medium text-gray-900">
                {new Date(recommendation.updatedAt).toLocaleString()}
              </dd>
            </div>
          </dl>
        </div>

        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Recommendation Summary</h2>
          <dl className="space-y-3">
            <div className="flex justify-between items-center">
              <dt className="text-sm text-gray-500">Confidence</dt>
              <dd className="flex items-center gap-2">
                <div className="w-24 bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-blue-600 h-2 rounded-full"
                    style={{ width: `${recommendation.confidence * 100}%` }}
                  />
                </div>
                <span className="text-sm font-medium text-gray-900">
                  {formatConfidence(recommendation.confidence)}
                </span>
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Estimated Savings</dt>
              <dd className="text-sm font-medium text-green-600">
                {formatCurrency(recommendation.estimatedSavings)}/month
              </dd>
            </div>
          </dl>
          {recommendation.reasoning && (
            <div className="mt-4 pt-4 border-t border-gray-200">
              <h3 className="text-sm font-medium text-gray-700 mb-2">Reasoning</h3>
              <p className="text-sm text-gray-600">{recommendation.reasoning}</p>
            </div>
          )}
        </div>
      </div>

      <ResourceComparison
        currentCpu={recommendation.currentCpu}
        recommendedCpu={recommendation.recommendedCpu}
        currentMemory={recommendation.currentMemory}
        recommendedMemory={recommendation.recommendedMemory}
      />

      {canTakeAction && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Actions</h2>
          <div className="flex gap-4">
            <button
              onClick={handleApprove}
              disabled={actionLoading !== null}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {actionLoading === 'approve' && (
                <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent" />
              )}
              Approve
            </button>
            <button
              onClick={handleDismiss}
              disabled={actionLoading !== null}
              className="px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {actionLoading === 'dismiss' && (
                <div className="animate-spin rounded-full h-4 w-4 border-2 border-gray-500 border-t-transparent" />
              )}
              Dismiss
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
