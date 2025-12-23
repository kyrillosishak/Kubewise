import { createContext, useContext, useEffect, type ReactNode } from 'react'
import { useAppDispatch, useAppSelector } from '@/store/hooks'
import { setUser, setToken, setPermissions, setLoading, logout as logoutAction } from '@/store/slices/authSlice'
import type { User, Permission } from '@/types/auth'
import { authStorage } from '@/lib/authStorage'
import { oidcClient } from '@/lib/oidc'
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

interface AuthContextValue {
  user: User | null
  token: string | null
  permissions: Permission[]
  loading: boolean
  isAuthenticated: boolean
  login: (credentials: LoginCredentials) => Promise<void>
  loginWithSSO: (returnTo?: string) => Promise<void>
  logout: (useSSOLogout?: boolean) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

interface AuthProviderProps {
  children: ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const dispatch = useAppDispatch()
  const { user, token, permissions, loading } = useAppSelector((state) => state.auth)

  const isAuthenticated = !!token && !!user

  const login = async (credentials: LoginCredentials): Promise<void> => {
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
  }

  const loginWithSSO = async (returnTo?: string): Promise<void> => {
    await oidcClient.startLogin(returnTo)
  }

  const logout = (useSSOLogout = false) => {
    authStorage.clearToken()
    dispatch(logoutAction())
    
    if (useSSOLogout && oidcClient.isConfigured()) {
      oidcClient.startLogout()
    }
  }

  useEffect(() => {
    const initializeAuth = async () => {
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
    }

    initializeAuth()
  }, [dispatch])

  return (
    <AuthContext.Provider value={{ user, token, permissions, loading, isAuthenticated, login, loginWithSSO, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuthContext() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuthContext must be used within an AuthProvider')
  }
  return context
}
