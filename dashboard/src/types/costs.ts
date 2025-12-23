export type Period = '7d' | '30d' | '90d'

export interface CostData {
  labels: string[]
  datasets: CostDataset[]
  total: number
  change: number
}

export interface CostDataset {
  label: string
  data: number[]
  borderColor: string
  backgroundColor?: string
}

export interface NamespaceCost {
  namespace: string
  currentCost: number
  previousCost: number
  change: number
  containers: number
}

export interface SavingsData {
  realized: number
  projected: number
  trend: SavingsTrend[]
}

export interface SavingsTrend {
  date: string
  realized: number
  projected: number
}
