import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from '@/store'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Login } from '@/pages/Login'
import { AuthCallback } from '@/pages/AuthCallback'
import { RecommendationList, RecommendationDetail } from '@/components/recommendations'
import { CostChart, NamespaceCostTable, SavingsSummary } from '@/components/costs'

function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <AuthProvider>
          <div className="min-h-screen bg-gray-50">
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route path="/auth/callback" element={<AuthCallback />} />
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <div className="p-8">Dashboard Home</div>
                  </ProtectedRoute>
                }
              />
              <Route
                path="/recommendations"
                element={
                  <ProtectedRoute>
                    <RecommendationList />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/recommendations/:id"
                element={
                  <ProtectedRoute>
                    <RecommendationDetail />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/costs"
                element={
                  <ProtectedRoute>
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
                  </ProtectedRoute>
                }
              />
              <Route
                path="/costs/namespace/:ns"
                element={
                  <ProtectedRoute>
                    <div className="p-8">Namespace Costs</div>
                  </ProtectedRoute>
                }
              />
              <Route
                path="/anomalies"
                element={
                  <ProtectedRoute>
                    <div className="p-8">Anomaly History</div>
                  </ProtectedRoute>
                }
              />
              <Route
                path="/clusters"
                element={
                  <ProtectedRoute>
                    <div className="p-8">Cluster Health</div>
                  </ProtectedRoute>
                }
              />
              <Route
                path="/settings"
                element={
                  <ProtectedRoute>
                    <div className="p-8">Settings</div>
                  </ProtectedRoute>
                }
              />
            </Routes>
          </div>
        </AuthProvider>
      </BrowserRouter>
    </Provider>
  )
}

export default App
