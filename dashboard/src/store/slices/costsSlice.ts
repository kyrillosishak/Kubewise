import { createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { CostData, Period } from '@/types/costs'

interface CostsState {
  data: CostData | null
  period: Period
  loading: boolean
}

const initialState: CostsState = {
  data: null,
  period: '30d',
  loading: false,
}

const costsSlice = createSlice({
  name: 'costs',
  initialState,
  reducers: {
    setCostData: (state, action: PayloadAction<CostData | null>) => {
      state.data = action.payload
    },
    setPeriod: (state, action: PayloadAction<Period>) => {
      state.period = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
  },
})

export const { setCostData, setPeriod, setLoading } = costsSlice.actions
export default costsSlice.reducer
