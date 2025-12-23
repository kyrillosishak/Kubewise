export interface Recommendation {
  id: string
  namespace: string
  deployment: string
  container: string
  confidence: number
  estimatedSavings: number
  currentCpu: string
  recommendedCpu: string
  currentMemory: string
  recommendedMemory: string
  status: 'pending' | 'approved' | 'dismissed' | 'applied'
  createdAt: string
  updatedAt: string
}

export interface RecommendationFilters {
  namespace?: string
  confidence?: number
  resourceType?: 'cpu' | 'memory' | 'both'
  status?: Recommendation['status']
}

export interface RecommendationDetail extends Recommendation {
  history: ResourceHistory[]
  reasoning: string
}

export interface ResourceHistory {
  timestamp: string
  cpu: number
  memory: number
}
