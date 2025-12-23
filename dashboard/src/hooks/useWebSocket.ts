import { useEffect, useState, useCallback } from 'react'
import { useAppDispatch } from '@/store/hooks'
import {
  getWebSocketClient,
  type ConnectionStatus,
  type WebSocketMessageType,
} from '@/api/websocket'

export function useWebSocket() {
  const dispatch = useAppDispatch()
  const [status, setStatus] = useState<ConnectionStatus>('disconnected')

  useEffect(() => {
    const client = getWebSocketClient({
      onStatusChange: setStatus,
    })

    client.connect(dispatch)

    return () => {
      client.disconnect()
    }
  }, [dispatch])

  const subscribe = useCallback(
    <T>(type: WebSocketMessageType, handler: (payload: T) => void) => {
      const client = getWebSocketClient()
      return client.subscribe(type, handler)
    },
    []
  )

  const send = useCallback(<T>(type: string, payload: T) => {
    const client = getWebSocketClient()
    client.send(type, payload)
  }, [])

  return {
    status,
    isConnected: status === 'connected',
    isReconnecting: status === 'reconnecting',
    subscribe,
    send,
  }
}

export default useWebSocket
