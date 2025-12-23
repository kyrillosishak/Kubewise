import { createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { Cluster } from '@/types/clusters'

interface ClustersState {
  items: Cluster[]
  selected: string | null
  loading: boolean
}

const initialState: ClustersState = {
  items: [],
  selected: null,
  loading: false,
}

const clustersSlice = createSlice({
  name: 'clusters',
  initialState,
  reducers: {
    setClusters: (state, action: PayloadAction<Cluster[]>) => {
      state.items = action.payload
    },
    updateCluster: (state, action: PayloadAction<Cluster>) => {
      const index = state.items.findIndex((c) => c.id === action.payload.id)
      if (index !== -1) {
        state.items[index] = action.payload
      } else {
        state.items.push(action.payload)
      }
    },
    setSelected: (state, action: PayloadAction<string | null>) => {
      state.selected = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
  },
})

export const { setClusters, updateCluster, setSelected, setLoading } = clustersSlice.actions
export default clustersSlice.reducer
