interface ResourceComparisonProps {
  currentCpu: string
  recommendedCpu: string
  currentMemory: string
  recommendedMemory: string
}

interface ResourceBarProps {
  label: string
  current: string
  recommended: string
  color: 'blue' | 'purple'
}

function parseResourceValue(value: string): number {
  const match = value.match(/^(\d+(?:\.\d+)?)(m|Mi|Gi|Ki)?$/)
  if (!match) return 0
  const num = parseFloat(match[1])
  const unit = match[2] || ''
  switch (unit) {
    case 'm':
      return num
    case 'Ki':
      return num * 1024
    case 'Mi':
      return num * 1024 * 1024
    case 'Gi':
      return num * 1024 * 1024 * 1024
    default:
      return num
  }
}

function ResourceBar({ label, current, recommended, color }: ResourceBarProps) {
  const currentVal = parseResourceValue(current)
  const recommendedVal = parseResourceValue(recommended)
  const maxVal = Math.max(currentVal, recommendedVal)
  const currentPercent = maxVal > 0 ? (currentVal / maxVal) * 100 : 0
  const recommendedPercent = maxVal > 0 ? (recommendedVal / maxVal) * 100 : 0

  const isReduction = recommendedVal < currentVal
  const changePercent = currentVal > 0 ? Math.round(((recommendedVal - currentVal) / currentVal) * 100) : 0

  const colorClasses = {
    blue: {
      current: 'bg-blue-400',
      recommended: 'bg-blue-600',
      text: 'text-blue-600',
    },
    purple: {
      current: 'bg-purple-400',
      recommended: 'bg-purple-600',
      text: 'text-purple-600',
    },
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <span
          className={`text-sm font-medium ${isReduction ? 'text-green-600' : 'text-orange-600'}`}
        >
          {changePercent > 0 ? '+' : ''}
          {changePercent}%
        </span>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 w-20">Current</span>
          <div className="flex-1 bg-gray-100 rounded-full h-4 relative overflow-hidden">
            <div
              className={`h-full rounded-full ${colorClasses[color].current} transition-all duration-300`}
              style={{ width: `${currentPercent}%` }}
            />
          </div>
          <span className="text-sm font-mono text-gray-700 w-24 text-right">{current}</span>
        </div>

        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 w-20">Recommended</span>
          <div className="flex-1 bg-gray-100 rounded-full h-4 relative overflow-hidden">
            <div
              className={`h-full rounded-full ${colorClasses[color].recommended} transition-all duration-300`}
              style={{ width: `${recommendedPercent}%` }}
            />
          </div>
          <span className={`text-sm font-mono w-24 text-right ${colorClasses[color].text}`}>
            {recommended}
          </span>
        </div>
      </div>
    </div>
  )
}

export function ResourceComparison({
  currentCpu,
  recommendedCpu,
  currentMemory,
  recommendedMemory,
}: ResourceComparisonProps) {
  const cpuReduction = parseResourceValue(currentCpu) - parseResourceValue(recommendedCpu)
  const memoryReduction = parseResourceValue(currentMemory) - parseResourceValue(recommendedMemory)

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-medium text-gray-900">Resource Comparison</h2>
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 rounded bg-blue-400" />
            <span className="text-gray-500">Current</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 rounded bg-blue-600" />
            <span className="text-gray-500">Recommended</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        <ResourceBar
          label="CPU Request"
          current={currentCpu}
          recommended={recommendedCpu}
          color="blue"
        />
        <ResourceBar
          label="Memory Request"
          current={currentMemory}
          recommended={recommendedMemory}
          color="purple"
        />
      </div>

      <div className="mt-6 pt-6 border-t border-gray-200">
        <h3 className="text-sm font-medium text-gray-700 mb-3">Summary</h3>
        <div className="grid grid-cols-2 gap-4">
          <div className="bg-gray-50 rounded-lg p-3">
            <div className="text-xs text-gray-500 mb-1">CPU Change</div>
            <div className={`text-lg font-semibold ${cpuReduction > 0 ? 'text-green-600' : 'text-orange-600'}`}>
              {cpuReduction > 0 ? '↓' : '↑'} {currentCpu} → {recommendedCpu}
            </div>
          </div>
          <div className="bg-gray-50 rounded-lg p-3">
            <div className="text-xs text-gray-500 mb-1">Memory Change</div>
            <div className={`text-lg font-semibold ${memoryReduction > 0 ? 'text-green-600' : 'text-orange-600'}`}>
              {memoryReduction > 0 ? '↓' : '↑'} {currentMemory} → {recommendedMemory}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
