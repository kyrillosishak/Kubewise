export interface ApiError {
  error: string
  code?: string
  details?: Record<string, unknown>
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface ApiResponse<T> {
  data: T
  status: number
}

export interface ApplyRequest {
  dryRun?: boolean
}

export interface ApplyResponse {
  id: string
  status: string
  message: string
  yamlPatch?: string
}

export interface ApproveRequest {
  approver: string
  reason?: string
}

export interface ApproveResponse {
  id: string
  status: string
  message: string
  approver: string
  approvedAt: string
}

export interface DryRunResponse {
  id: string
  wouldApply: boolean
  yamlPatch: string
  currentResources: ResourceSpec
  recommendedResources: ResourceSpec
}

export interface ResourceSpec {
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
}

export interface CostImpact {
  currentMonthlyCost: number
  projectedMonthlyCost: number
  monthlySavings: number
  currency: string
}

export interface SafetyConfig {
  namespace: string
  dryRunEnabled: boolean
  requireApproval: boolean
  approvalThreshold: number
  autoApplyEnabled: boolean
  autoApplyMaxRisk: 'low' | 'medium' | 'high'
  autoApplyMinConfidence: number
}

export interface Model {
  version: string
  createdAt: string
  validationAccuracy: number
  isActive: boolean
  sizeBytes: number
  deployedAgents: number
  totalAgents: number
}

export interface PredictionHistoryEntry {
  timestamp: string
  cpuRequestMillicores: number
  memoryRequestBytes: number
  confidence: number
  modelVersion: string
}

export interface PredictionHistory {
  deployment: string
  namespace: string
  predictions: PredictionHistoryEntry[]
}
