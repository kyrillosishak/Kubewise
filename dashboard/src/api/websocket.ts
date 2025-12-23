import type { AppDispatch } from '@/store'
import { updateRecommendation } from '@/store/slices/recommendationsSlice'
import { addAnomaly, updateAnomaly } from '@/store/slices/anomaliesSlice'
import { updateCluster } from '@/store/slices/clustersSlice'
import type { Recommendation, Anomaly, Cluster } from '@/types'
import { authStorage } from '@/lib/authStorage'

export type WebSocketMessageType =
  | 'recommendation_created'
  | 'recommendation_updated'
  | 'anomaly_detected'
  | 'anomaly_resolved'
  | 'cluster_status_changed'
  | 'connected'
  | 'error'

export interface WebSocketMessage<T = unknown> {
  type: WebSocketMessageType
  payload: T
  timestamp: string
}

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting'

export interface WebSocketClientOptions {
  url?: string
  maxReconnectAttempts?: number
  initialReconnectDelay?: number
  maxReconnectDelay?: number
  onStatusChange?: (status: ConnectionStatus) => void
}

const DEFAULT_OPTIONS: Required<Omit<WebSocketClientOptions, 'onStatusChange'>> = {
  url: `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws`,
  maxReconnectAttempts: 10,
  initialReconnectDelay: 1000,
  maxReconnectDelay: 30000,
}

export class WebSocketClient {
  private ws: WebSocket | null = null
  private dispatch: AppDispatch | null = null
  private options: Required<Omit<WebSocketClientOptions, 'onStatusChange'>>
  private onStatusChange?: (status: ConnectionStatus) => void
  private reconnectAttempts = 0
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  private status: ConnectionStatus = 'disconnected'
  private messageHandlers: Map<WebSocketMessageType, Set<(payload: unknown) => void>> = new Map()

  constructor(options: WebSocketClientOptions = {}) {
    this.options = { ...DEFAULT_OPTIONS, ...options }
    this.onStatusChange = options.onStatusChange
  }

  connect(dispatch: AppDispatch): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    this.dispatch = dispatch
    this.setStatus('connecting')

    const token = authStorage.getToken()
    const url = token ? `${this.options.url}?token=${encodeURIComponent(token)}` : this.options.url

    try {
      this.ws = new WebSocket(url)
      this.setupEventHandlers()
    } catch (error) {
      console.error('WebSocket connection failed:', error)
      this.scheduleReconnect()
    }
  }

  private setupEventHandlers(): void {
    if (!this.ws) return

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      this.setStatus('connected')
    }

    this.ws.onclose = (event) => {
      if (event.code !== 1000) {
        this.scheduleReconnect()
      } else {
        this.setStatus('disconnected')
      }
    }

    this.ws.onerror = () => {
      this.setStatus('disconnected')
    }

    this.ws.onmessage = (event) => {
      this.handleMessage(event.data)
    }
  }

  private handleMessage(data: string): void {
    try {
      const message = JSON.parse(data) as WebSocketMessage
      this.dispatchToStore(message)
      this.notifyHandlers(message)
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error)
    }
  }

  private dispatchToStore(message: WebSocketMessage): void {
    if (!this.dispatch) return

    switch (message.type) {
      case 'recommendation_created':
      case 'recommendation_updated':
        this.dispatch(updateRecommendation(message.payload as Recommendation))
        break

      case 'anomaly_detected':
        this.dispatch(addAnomaly(message.payload as Anomaly))
        break

      case 'anomaly_resolved':
        this.dispatch(updateAnomaly(message.payload as Anomaly))
        break

      case 'cluster_status_changed':
        this.dispatch(updateCluster(message.payload as Cluster))
        break
    }
  }

  private notifyHandlers(message: WebSocketMessage): void {
    const handlers = this.messageHandlers.get(message.type)
    if (handlers) {
      handlers.forEach((handler) => handler(message.payload))
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
      this.setStatus('disconnected')
      return
    }

    this.setStatus('reconnecting')
    const delay = Math.min(
      this.options.initialReconnectDelay * Math.pow(2, this.reconnectAttempts),
      this.options.maxReconnectDelay
    )

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectAttempts++
      if (this.dispatch) {
        this.connect(this.dispatch)
      }
    }, delay)
  }

  private setStatus(status: ConnectionStatus): void {
    this.status = status
    this.onStatusChange?.(status)
  }

  getStatus(): ConnectionStatus {
    return this.status
  }

  subscribe<T>(type: WebSocketMessageType, handler: (payload: T) => void): () => void {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, new Set())
    }
    this.messageHandlers.get(type)!.add(handler as (payload: unknown) => void)

    return () => {
      this.messageHandlers.get(type)?.delete(handler as (payload: unknown) => void)
    }
  }

  send<T>(type: string, payload: T): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload, timestamp: new Date().toISOString() }))
    }
  }

  disconnect(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout)
      this.reconnectTimeout = null
    }

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect')
      this.ws = null
    }

    this.setStatus('disconnected')
    this.reconnectAttempts = 0
  }
}

let wsClientInstance: WebSocketClient | null = null

export function getWebSocketClient(options?: WebSocketClientOptions): WebSocketClient {
  if (!wsClientInstance) {
    wsClientInstance = new WebSocketClient(options)
  }
  return wsClientInstance
}

export default WebSocketClient
