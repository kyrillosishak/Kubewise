export type ClusterStatus = 'healthy' | 'degraded' | 'disconnected'

export interface Cluster {
  id: string
  name: string
  status: ClusterStatus
  containersMonitored: number
  predictionsGenerated: number
  anomaliesDetected: number
  modelVersion: string
  lastSeen: string
}

export interface ClusterHealth {
  cluster: Cluster
  agents: AgentStatus[]
  metrics: ClusterMetrics
}

export interface AgentStatus {
  nodeId: string
  nodeName: string
  status: 'running' | 'stopped' | 'error'
  lastHeartbeat: string
  version: string
}

export interface ClusterMetrics {
  cpuUtilization: number
  memoryUtilization: number
  podCount: number
  nodeCount: number
}
