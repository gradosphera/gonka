import { useEffect, useState } from 'react'
import { InferenceResponse } from './types/inference'
import { ParticipantTable } from './components/ParticipantTable'
import { EpochSelector } from './components/EpochSelector'

function App() {
  const [data, setData] = useState<InferenceResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string>('')
  const [selectedEpochId, setSelectedEpochId] = useState<number | null>(null)
  const [currentEpochId, setCurrentEpochId] = useState<number | null>(null)
  const [selectedParticipantId, setSelectedParticipantId] = useState<string | null>(null)
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
      
      if (result.is_current) {
        setCurrentEpochId(result.epoch_id)
      }
      
      setAutoRefreshCountdown(30)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const epochParam = params.get('epoch')
    const participantParam = params.get('participant')
    
    if (epochParam) {
      const epochId = parseInt(epochParam)
      if (!isNaN(epochId)) {
        setSelectedEpochId(epochId)
        if (participantParam) {
          setSelectedParticipantId(participantParam)
        }
        return
      }
    }
    
    if (participantParam) {
      setSelectedParticipantId(participantParam)
    }
    
    fetchData(null)
  }, [])

  useEffect(() => {
    fetchData(selectedEpochId)
    
    const params = new URLSearchParams(window.location.search)
    if (selectedEpochId === null) {
      params.delete('epoch')
    } else {
      params.set('epoch', selectedEpochId.toString())
    }
    
    const newUrl = params.toString() 
      ? `${window.location.pathname}?${params.toString()}`
      : window.location.pathname
    window.history.replaceState({}, '', newUrl)
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
  
  const handleParticipantSelect = (participantId: string | null) => {
    setSelectedParticipantId(participantId)
    
    const params = new URLSearchParams(window.location.search)
    if (participantId) {
      params.set('participant', participantId)
    } else {
      params.delete('participant')
    }
    
    const newUrl = params.toString() ? `?${params.toString()}` : window.location.pathname
    window.history.replaceState({}, '', newUrl)
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
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-8 max-w-[1600px]">
        <header className="mb-8 flex items-center gap-4">
          <img src="/gonka.svg" alt="Gonka" className="h-12 w-auto" />
          <div>
            <h1 className="text-4xl font-bold text-gray-900 mb-1">
              Gonka Chain Inference Tracker
            </h1>
            <p className="text-base text-gray-600">
              Real-time monitoring of participant performance and model availability
            </p>
          </div>
        </header>

        {data && (
          <>
            <div className="bg-white rounded-lg shadow-sm p-6 mb-6 border border-gray-200">
              <div className="flex flex-wrap items-center justify-between gap-6">
                <div className="flex flex-wrap items-center gap-8">
                  <div>
                    <div className="text-sm font-medium text-gray-500 mb-1">Epoch ID</div>
                    <div className="flex items-center gap-2">
                      <span className="text-2xl font-bold text-gray-900">
                        {data.epoch_id}
                      </span>
                      {data.is_current && (
                        <span className="px-2.5 py-0.5 text-xs font-semibold bg-gray-900 text-white rounded">
                          CURRENT
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="border-l border-gray-200 pl-8">
                    <div className="text-sm font-medium text-gray-500 mb-1">Block Height</div>
                    <div className="text-2xl font-bold text-gray-900">
                      {data.height.toLocaleString()}
                    </div>
                  </div>

                  <div className="border-l border-gray-200 pl-8">
                    <div className="text-sm font-medium text-gray-500 mb-1">Total Participants</div>
                    <div className="text-2xl font-bold text-gray-900">
                      {data.participants.length}
                    </div>
                  </div>

                  <div className="border-l border-gray-200 pl-8">
                    <div className="text-sm font-medium text-gray-500 mb-1">Total Assigned Rewards</div>
                    <div className="text-2xl font-bold text-gray-900">
                      {data.total_assigned_rewards_gnk !== undefined && data.total_assigned_rewards_gnk !== null 
                        ? `${data.total_assigned_rewards_gnk.toLocaleString()} GNK`
                        : <span className="text-gray-400 italic">Calculating...</span>
                      }
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <EpochSelector
                    currentEpochId={currentEpochId || data.epoch_id}
                    selectedEpochId={selectedEpochId}
                    onSelectEpoch={handleEpochSelect}
                    disabled={loading}
                  />
                  <button
                    onClick={handleRefresh}
                    disabled={loading}
                    className="px-5 py-2.5 bg-gray-900 text-white font-medium rounded-md hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                  >
                    {loading ? 'Refreshing...' : 'Refresh'}
                  </button>
                </div>
              </div>

              {selectedEpochId === null && (
                <div className="mt-4 pt-4 border-t border-gray-200 flex items-center justify-end text-xs text-gray-500">
                  <span>Auto-refresh in {autoRefreshCountdown}s</span>
                </div>
              )}
            </div>

            <div className="bg-white rounded-lg shadow-sm p-6 border border-gray-200">
              <div className="mb-4">
                <h2 className="text-xl font-bold text-gray-900 mb-1">
                  Participant Statistics
                </h2>
                <p className="text-sm text-gray-500">
                  Rows with red background indicate missed rate or invalidation rate exceeding 10%
                </p>
              </div>
              <ParticipantTable 
                participants={data.participants} 
                epochId={data.epoch_id}
                selectedParticipantId={selectedParticipantId}
                onParticipantSelect={handleParticipantSelect}
              />
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default App
