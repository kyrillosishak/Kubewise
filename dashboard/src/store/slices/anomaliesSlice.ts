import { createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { Anomaly, AnomalyFilters } from '@/types/anomalies'

interface AnomaliesState {
  items: Anomaly[]
  filters: AnomalyFilters
  loading: boolean
}

const initialState: AnomaliesState = {
  items: [],
  filters: {},
  loading: false,
}

const anomaliesSlice = createSlice({
  name: 'anomalies',
  initialState,
  reducers: {
    setAnomalies: (state, action: PayloadAction<Anomaly[]>) => {
      state.items = action.payload
    },
    addAnomaly: (state, action: PayloadAction<Anomaly>) => {
      const exists = state.items.some((a) => a.id === action.payload.id)
      if (!exists) {
        state.items.unshift(action.payload)
      }
    },
    updateAnomaly: (state, action: PayloadAction<Anomaly>) => {
      const index = state.items.findIndex((a) => a.id === action.payload.id)
      if (index !== -1) {
        state.items[index] = action.payload
      }
    },
    setFilters: (state, action: PayloadAction<AnomalyFilters>) => {
      state.filters = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
  },
})

export const { setAnomalies, addAnomaly, updateAnomaly, setFilters, setLoading } =
  anomaliesSlice.actions
export default anomaliesSlice.reducer
