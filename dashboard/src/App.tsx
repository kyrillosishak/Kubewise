import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from '@/store'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Login } from '@/pages/Login'
import { AuthCallback } from '@/pages/AuthCallback'
import { RecommendationList, RecommendationDetail } from '@/components/recommendations'

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
                    <div className="p-8">Cost Analytics</div>
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
