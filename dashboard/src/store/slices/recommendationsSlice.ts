import { createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { Recommendation, RecommendationFilters } from '@/types/recommendations'

interface RecommendationsState {
  items: Recommendation[]
  loading: boolean
  filters: RecommendationFilters
}

const initialState: RecommendationsState = {
  items: [],
  loading: false,
  filters: {},
}

const recommendationsSlice = createSlice({
  name: 'recommendations',
  initialState,
  reducers: {
    setRecommendations: (state, action: PayloadAction<Recommendation[]>) => {
      state.items = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
    setFilters: (state, action: PayloadAction<RecommendationFilters>) => {
      state.filters = action.payload
    },
    updateRecommendation: (state, action: PayloadAction<Recommendation>) => {
      const index = state.items.findIndex((r) => r.id === action.payload.id)
      if (index !== -1) {
        state.items[index] = action.payload
      }
    },
  },
})

export const { setRecommendations, setLoading, setFilters, updateRecommendation } =
  recommendationsSlice.actions
export default recommendationsSlice.reducer
