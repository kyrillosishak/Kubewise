import { configureStore } from '@reduxjs/toolkit'
import { useDispatch, useSelector, type TypedUseSelectorHook } from 'react-redux'
import authReducer from './slices/authSlice'
import recommendationsReducer from './slices/recommendationsSlice'
import costsReducer from './slices/costsSlice'
import anomaliesReducer from './slices/anomaliesSlice'
import clustersReducer from './slices/clustersSlice'

export const store = configureStore({
  reducer: {
    auth: authReducer,
    recommendations: recommendationsReducer,
    costs: costsReducer,
    anomalies: anomaliesReducer,
    clusters: clustersReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch

export const useAppDispatch: () => AppDispatch = useDispatch
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector
