import { createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { User, Permission } from '@/types/auth'

interface AuthState {
  user: User | null
  token: string | null
  permissions: Permission[]
  loading: boolean
}

const initialState: AuthState = {
  user: null,
  token: null,
  permissions: [],
  loading: true, // Start as true to prevent redirect before auth check
}

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setUser: (state, action: PayloadAction<User | null>) => {
      state.user = action.payload
    },
    setToken: (state, action: PayloadAction<string | null>) => {
      state.token = action.payload
    },
    setPermissions: (state, action: PayloadAction<Permission[]>) => {
      state.permissions = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
    logout: (state) => {
      state.user = null
      state.token = null
      state.permissions = []
    },
  },
})

export const { setUser, setToken, setPermissions, setLoading, logout } = authSlice.actions
export default authSlice.reducer
