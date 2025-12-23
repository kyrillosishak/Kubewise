import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAppDispatch } from '@/store/hooks'
import { setUser, setToken, setPermissions, setLoading } from '@/store/slices/authSlice'
import { oidcClient } from '@/lib/oidc'
import { authStorage } from '@/lib/authStorage'
import apiClient from '@/api/client'
import type { User, Permission } from '@/types/auth'

interface TokenExchangeResponse {
  user: User
  token: string
  expiresIn: number
  permissions: Permission[]
}

export function AuthCallback() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handleOAuthCallback = async () => {
      dispatch(setLoading(true))
      try {
        const { code, codeVerifier, nonce, returnTo } = await oidcClient.handleCallback(searchParams)

        const response = await apiClient.post<TokenExchangeResponse>('/auth/token', {
          code,
          codeVerifier,
          nonce,
          redirectUri: oidcClient.config.redirectUri,
        })

        authStorage.setToken(response.token, response.expiresIn)
        dispatch(setToken(response.token))
        dispatch(setUser(response.user))
        dispatch(setPermissions(response.permissions))

        navigate(returnTo || '/', { replace: true })
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Authentication failed'
        setError(message)
      } finally {
        dispatch(setLoading(false))
      }
    }

    handleOAuthCallback()
  }, [searchParams, navigate, dispatch])

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full p-6 bg-white rounded-lg shadow-md">
          <h2 className="text-xl font-semibold text-red-600 mb-2">Authentication Error</h2>
          <p className="text-gray-600 mb-4">{error}</p>
          <button
            onClick={() => navigate('/login')}
            className="w-full py-2 px-4 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            Return to Login
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4" />
        <p className="text-gray-600">Completing authentication...</p>
      </div>
    </div>
  )
}
