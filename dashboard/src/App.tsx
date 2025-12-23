import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from '@/store'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { MainLayout } from '@/components/layout'
import { Login, AuthCallback, Dashboard, Anomalies, Clusters } from '@/pages'
import { RecommendationList, RecommendationDetail } from '@/components/recommendations'
import { CostChart, NamespaceCostTable, SavingsSummary } from '@/components/costs'

function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/auth/callback" element={<AuthCallback />} />
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <Dashboard />
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/recommendations"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <RecommendationList />
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/recommendations/:id"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <RecommendationDetail />
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/costs"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <div className="p-6 space-y-6">
                      <h1 className="text-2xl font-semibold text-gray-900">Cost Analytics</h1>
                      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                        <div className="lg:col-span-2">
                          <CostChart />
                        </div>
                        <div>
                          <SavingsSummary />
                        </div>
                      </div>
                      <NamespaceCostTable />
                    </div>
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/costs/namespace/:ns"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <div className="p-8">Namespace Costs</div>
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/anomalies"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <Anomalies />
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/clusters"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <Clusters />
                  </MainLayout>
                </ProtectedRoute>
              }
            />
            <Route
              path="/settings"
              element={
                <ProtectedRoute>
                  <MainLayout>
                    <div className="p-8">
                      <h1 className="text-2xl font-semibold text-gray-900">Settings</h1>
                      <p className="mt-2 text-gray-500">Settings page coming soon.</p>
                    </div>
                  </MainLayout>
                </ProtectedRoute>
              }
            />
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </Provider>
  )
}

export default App
