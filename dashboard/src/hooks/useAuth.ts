import { useCallback, useEffect } from 'react'
import { useAppDispatch, useAppSelector } from '@/store/hooks'
import { setUser, setToken, setPermissions, setLoading, logout as logoutAction } from '@/store/slices/authSlice'
import type { User, Permission } from '@/types/auth'
import { authStorage } from '@/lib/authStorage'
import apiClient from '@/api/client'

interface LoginCredentials {
  email: string
  password: string
}

interface AuthResponse {
  user: User
  token: string
  expiresIn: number
  permissions: Permission[]
}

export function useAuth() {
  const dispatch = useAppDispatch()
  const { user, token, permissions, loading } = useAppSelector((state) => state.auth)

  const isAuthenticated = !!token && !!user

  const login = useCallback(async (credentials: LoginCredentials): Promise<void> => {
    dispatch(setLoading(true))
    try {
      const response = await apiClient.post<AuthResponse>('/auth/login', credentials)
      
      authStorage.setToken(response.token, response.expiresIn)
      dispatch(setToken(response.token))
      dispatch(setUser(response.user))
      dispatch(setPermissions(response.permissions))
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch])

  const logout = useCallback(() => {
    authStorage.clearToken()
    dispatch(logoutAction())
  }, [dispatch])

  const initializeAuth = useCallback(async () => {
    const storedToken = authStorage.getToken()
    if (!storedToken) {
      dispatch(setLoading(false))
      return
    }

    dispatch(setLoading(true))
    try {
      const response = await apiClient.get<{ user: User; permissions: Permission[] }>('/auth/me')
      dispatch(setToken(storedToken))
      dispatch(setUser(response.user))
      dispatch(setPermissions(response.permissions))
    } catch {
      authStorage.clearToken()
      dispatch(logoutAction())
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch])

  useEffect(() => {
    if (!user && !loading) {
      initializeAuth()
    }
  }, [])

  return {
    user,
    token,
    permissions,
    loading,
    isAuthenticated,
    login,
    logout,
    initializeAuth,
  }
}
