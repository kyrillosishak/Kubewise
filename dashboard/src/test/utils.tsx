import type { ReactElement, ReactNode } from 'react'
import { render, type RenderOptions } from '@testing-library/react'
import { Provider } from 'react-redux'
import { BrowserRouter } from 'react-router-dom'
import { configureStore } from '@reduxjs/toolkit'
import authReducer from '@/store/slices/authSlice'
import recommendationsReducer from '@/store/slices/recommendationsSlice'
import costsReducer from '@/store/slices/costsSlice'
import anomaliesReducer from '@/store/slices/anomaliesSlice'
import clustersReducer from '@/store/slices/clustersSlice'
import type { RootState } from '@/store'

interface ExtendedRenderOptions extends Omit<RenderOptions, 'queries'> {
  preloadedState?: Partial<RootState>
  withRouter?: boolean
}

export function createTestStore(preloadedState?: Partial<RootState>) {
  return configureStore({
    reducer: {
      auth: authReducer,
      recommendations: recommendationsReducer,
      costs: costsReducer,
      anomalies: anomaliesReducer,
      clusters: clustersReducer,
    },
    preloadedState: preloadedState as RootState,
  })
}

export function renderWithProviders(
  ui: ReactElement,
  {
    preloadedState,
    withRouter = true,
    ...renderOptions
  }: ExtendedRenderOptions = {}
) {
  const store = createTestStore(preloadedState)

  function Wrapper({ children }: { children: ReactNode }) {
    const content = <Provider store={store}>{children}</Provider>
    return withRouter ? <BrowserRouter>{content}</BrowserRouter> : content
  }

  return { store, ...render(ui, { wrapper: Wrapper, ...renderOptions }) }
}

export * from '@testing-library/react'
export { default as userEvent } from '@testing-library/user-event'
