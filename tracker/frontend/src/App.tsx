import { useEffect, useState } from 'react'
import { InferenceResponse } from './types/inference'
import { ParticipantTable } from './components/ParticipantTable'
import { EpochSelector } from './components/EpochSelector'

function App() {
  const [data, setData] = useState<InferenceResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string>('')
  const [selectedEpochId, setSelectedEpochId] = useState<number | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [autoRefreshCountdown, setAutoRefreshCountdown] = useState(30)

  const apiUrl = import.meta.env.VITE_API_URL || '/api'

  const fetchData = async (epochId: number | null = null, isAutoRefresh = false) => {
    if (!isAutoRefresh) {
      setLoading(true)
    }
    setError('')

    try {
      const endpoint = epochId
        ? `${apiUrl}/v1/inference/epochs/${epochId}`
        : `${apiUrl}/v1/inference/current`
      
      const response = await fetch(endpoint)
      
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      
      const result = await response.json()
      setData(result)
      setLastUpdated(new Date())
      setAutoRefreshCountdown(30)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData(selectedEpochId)
  }, [selectedEpochId])

  useEffect(() => {
    if (selectedEpochId !== null) return

    const interval = setInterval(() => {
      setAutoRefreshCountdown((prev) => {
        if (prev <= 1) {
          fetchData(null, true)
          return 30
        }
        return prev - 1
      })
    }, 1000)

    return () => clearInterval(interval)
  }, [selectedEpochId])

  const handleRefresh = () => {
    fetchData(selectedEpochId)
  }

  const handleEpochSelect = (epochId: number | null) => {
    setSelectedEpochId(epochId)
  }

  const formatTimestamp = (date: Date) => {
    const now = new Date()
    const diffSeconds = Math.floor((now.getTime() - date.getTime()) / 1000)
    
    if (diffSeconds < 60) return `${diffSeconds} seconds ago`
    if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)} minutes ago`
    return date.toLocaleTimeString()
  }

  if (loading && !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-blue-600 border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading inference statistics...</p>
        </div>
      </div>
    )
  }

  if (error && !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 max-w-md">
          <h2 className="text-red-800 text-lg font-semibold mb-2">Error</h2>
          <p className="text-red-600">{error}</p>
          <button
            onClick={handleRefresh}
            className="mt-4 px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100">
      <div className="container mx-auto px-4 py-8">
        <header className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900 mb-2">
            Gonka Chain Inference Statistics
          </h1>
          <p className="text-gray-600">
            Real-time monitoring of participant performance and model availability
          </p>
        </header>

        {data && (
          <>
            <div className="bg-white rounded-lg shadow-md p-6 mb-6">
              <div className="flex flex-wrap items-center justify-between gap-4">
                <div className="flex flex-wrap items-center gap-6">
                  <div>
                    <span className="text-sm text-gray-600">Epoch ID:</span>
                    <span className="ml-2 text-lg font-semibold text-gray-900">
                      {data.epoch_id}
                    </span>
                    {data.is_current && (
                      <span className="ml-2 px-2 py-1 text-xs bg-green-100 text-green-800 rounded">
                        CURRENT
                      </span>
                    )}
                  </div>
                  <div>
                    <span className="text-sm text-gray-600">Height:</span>
                    <span className="ml-2 text-lg font-semibold text-gray-900">
                      {data.height.toLocaleString()}
                    </span>
                  </div>
                  <div>
                    <span className="text-sm text-gray-600">Participants:</span>
                    <span className="ml-2 text-lg font-semibold text-gray-900">
                      {data.participants.length}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <EpochSelector
                    currentEpochId={data.epoch_id}
                    selectedEpochId={selectedEpochId}
                    onSelectEpoch={handleEpochSelect}
                    disabled={loading}
                  />
                  <button
                    onClick={handleRefresh}
                    disabled={loading}
                    className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                  >
                    {loading ? 'Refreshing...' : 'Refresh'}
                  </button>
                </div>
              </div>

              <div className="mt-4 flex items-center justify-between text-sm text-gray-500">
                <div>
                  {lastUpdated && (
                    <span>Last updated: {formatTimestamp(lastUpdated)}</span>
                  )}
                </div>
                <div>
                  {selectedEpochId === null && (
                    <span>Auto-refresh in {autoRefreshCountdown}s</span>
                  )}
                </div>
              </div>
            </div>

            <div className="bg-white rounded-lg shadow-md p-6">
              <h2 className="text-xl font-semibold text-gray-900 mb-4">
                Participant Statistics
              </h2>
              <p className="text-sm text-gray-600 mb-4">
                Participants highlighted in red have missed rate or invalidation rate exceeding 10%
              </p>
              <ParticipantTable participants={data.participants} />
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default App
