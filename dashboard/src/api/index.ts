import apiClient from './client'
import type {
  Recommendation,
  RecommendationDetail,
  RecommendationFilters,
  CostData,
  NamespaceCost,
  SavingsData,
  Period,
  Anomaly,
  AnomalyDetail,
  AnomalyFilters,
  Cluster,
  ClusterHealth,
  ApproveRequest,
  ApproveResponse,
  DryRunResponse,
  SafetyConfig,
  Model,
  PredictionHistory,
} from '@/types'

interface RecommendationListResponse {
  recommendations: Recommendation[]
  total: number
}

interface ModelListResponse {
  models: Model[]
}

export const api = {
  // Recommendations
  getRecommendations: async (filters?: RecommendationFilters): Promise<Recommendation[]> => {
    const response = await apiClient.get<RecommendationListResponse>('/v1/recommendations', {
      params: filters,
    })
    return response.recommendations
  },

  getRecommendation: async (id: string): Promise<RecommendationDetail> => {
    return apiClient.get<RecommendationDetail>(`/v1/recommendations/${id}`)
  },

  getRecommendationsByNamespace: async (namespace: string): Promise<Recommendation[]> => {
    const response = await apiClient.get<RecommendationListResponse>(
      `/v1/recommendations/${namespace}`
    )
    return response.recommendations
  },

  approveRecommendation: async (id: string, request: ApproveRequest): Promise<ApproveResponse> => {
    return apiClient.post<ApproveResponse, ApproveRequest>(
      `/v1/recommendation/${id}/approve`,
      request
    )
  },

  dismissRecommendation: async (id: string): Promise<void> => {
    await apiClient.post(`/v1/recommendation/${id}/dismiss`)
  },

  dryRunRecommendation: async (id: string): Promise<DryRunResponse> => {
    return apiClient.post<DryRunResponse>(`/v1/recommendation/${id}/dry-run`)
  },

  // Costs
  getCosts: async (period: Period): Promise<CostData> => {
    return apiClient.get<CostData>('/v1/costs', { params: { period } })
  },

  getNamespaceCosts: async (namespace: string): Promise<NamespaceCost> => {
    return apiClient.get<NamespaceCost>(`/v1/costs/${namespace}`)
  },

  getSavings: async (since: string = '30d'): Promise<SavingsData> => {
    return apiClient.get<SavingsData>('/v1/savings', { params: { since } })
  },

  // Anomalies
  getAnomalies: async (filters?: AnomalyFilters): Promise<Anomaly[]> => {
    return apiClient.get<Anomaly[]>('/v1/anomalies', { params: filters })
  },

  getAnomalyDetail: async (id: string): Promise<AnomalyDetail> => {
    return apiClient.get<AnomalyDetail>(`/v1/anomalies/${id}`)
  },

  // Clusters
  getClusters: async (): Promise<Cluster[]> => {
    return apiClient.get<Cluster[]>('/v1/clusters')
  },

  getClusterHealth: async (id: string): Promise<ClusterHealth> => {
    return apiClient.get<ClusterHealth>(`/v1/clusters/${id}/health`)
  },

  // Models
  getModels: async (): Promise<Model[]> => {
    const response = await apiClient.get<ModelListResponse>('/v1/models')
    return response.models
  },

  getModel: async (version: string): Promise<Model> => {
    return apiClient.get<Model>(`/v1/models/${version}`)
  },

  rollbackModel: async (version: string): Promise<{ success: boolean; message: string }> => {
    return apiClient.post(`/v1/models/rollback/${version}`)
  },

  // Safety
  getSafetyConfig: async (namespace: string): Promise<SafetyConfig> => {
    return apiClient.get<SafetyConfig>(`/v1/safety/config/${namespace}`)
  },

  updateSafetyConfig: async (namespace: string, config: SafetyConfig): Promise<void> => {
    await apiClient.put(`/v1/safety/config/${namespace}`, config)
  },

  // Debug
  getPredictionHistory: async (
    deployment: string,
    namespace?: string,
    since: string = '24h'
  ): Promise<PredictionHistory> => {
    return apiClient.get<PredictionHistory>(`/v1/debug/predictions/${deployment}`, {
      params: { namespace, since },
    })
  },
}

export { apiClient, ApiRequestError } from './client'
export default api
