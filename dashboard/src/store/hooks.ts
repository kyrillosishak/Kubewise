import { useDispatch, useSelector, type TypedUseSelectorHook } from 'react-redux'
import type { store } from './index'

type RootState = ReturnType<typeof store.getState>
type AppDispatch = typeof store.dispatch

export const useAppDispatch: () => AppDispatch = useDispatch
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector
