import { useState } from 'react'
import { ClusterHealthGrid, ClusterDetail } from '@/components/clusters'

export function Clusters() {
  const [selectedClusterId, setSelectedClusterId] = useState<string | null>(null)

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900">Cluster Health</h1>
        {selectedClusterId && (
          <button
            onClick={() => setSelectedClusterId(null)}
            className="text-sm text-blue-600 hover:text-blue-800 flex items-center gap-1"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to all clusters
          </button>
        )}
      </div>

      {selectedClusterId ? (
        <ClusterDetail clusterId={selectedClusterId} />
      ) : (
        <ClusterHealthGrid onSelectCluster={setSelectedClusterId} />
      )}
    </div>
  )
}
