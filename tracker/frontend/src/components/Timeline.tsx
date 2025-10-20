import { useEffect, useState } from 'react'
import { TimelineResponse } from '../types/inference'

export function Timeline() {
  const [data, setData] = useState<TimelineResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string>('')
  const [hoveredBlock, setHoveredBlock] = useState<number | null>(null)
  const [hoveredEpoch, setHoveredEpoch] = useState<number | null>(null)
  const [mousePosition, setMousePosition] = useState<{ x: number; y: number } | null>(null)

  const apiUrl = import.meta.env.VITE_API_URL || '/api'

  useEffect(() => {
    const fetchTimeline = async () => {
      setLoading(true)
      setError('')

      try {
        const response = await fetch(`${apiUrl}/v1/timeline`)
        
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`)
        }
        
        const result = await response.json()
        setData(result)
        
        const params = new URLSearchParams(window.location.search)
        const blockParam = params.get('block')
        
        if (blockParam) {
          const blockHeight = parseInt(blockParam)
          if (!isNaN(blockHeight)) {
            setHoveredBlock(blockHeight)
          }
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch timeline data')
      } finally {
        setLoading(false)
      }
    }

    fetchTimeline()
  }, [apiUrl])

  const calculateBlockTime = (blockHeight: number): string => {
    if (!data) return ''

    const currentHeight = data.current_block.height
    const currentTime = new Date(data.current_block.timestamp).getTime()
    const blockDiff = blockHeight - currentHeight
    const timeDiff = blockDiff * data.avg_block_time * 1000

    const estimatedTime = new Date(currentTime + timeDiff)
    return estimatedTime.toUTCString()
  }

  const handleTimelineClick = (blockHeight: number) => {
    setHoveredBlock(blockHeight)
    const params = new URLSearchParams(window.location.search)
    params.set('block', blockHeight.toString())
    window.history.replaceState({}, '', `?${params.toString()}`)
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-blue-600 border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading timeline...</p>
        </div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 max-w-md">
          <h2 className="text-red-800 text-lg font-semibold mb-2">Error</h2>
          <p className="text-red-600">{error || 'No data available'}</p>
        </div>
      </div>
    )
  }

  const minBlock = data.reference_block.height
  
  const twoMonthsInSeconds = 60 * 24 * 3600
  const blocksInTwoMonths = Math.ceil(twoMonthsInSeconds / data.avg_block_time)
  
  let maxBlock = data.current_block.height + blocksInTwoMonths
  
  const maxEventBlock = Math.max(...data.events.map(e => e.block_height))
  if (maxEventBlock > maxBlock) {
    maxBlock = maxEventBlock + Math.floor(blocksInTwoMonths * 0.1)
  }
  
  const blockRange = maxBlock - minBlock

  const getEpochData = () => {
    const epochs: Array<{ block: number; epochNumber: number }> = []
    
    let epochStart = data.current_epoch_start
    let epochNum = data.current_epoch_index
    
    while (epochStart >= minBlock) {
      epochs.push({ block: epochStart, epochNumber: epochNum })
      epochStart -= data.epoch_length
      epochNum--
    }
    
    epochStart = data.current_epoch_start + data.epoch_length
    epochNum = data.current_epoch_index + 1
    while (epochStart <= maxBlock) {
      epochs.push({ block: epochStart, epochNumber: epochNum })
      epochStart += data.epoch_length
      epochNum++
    }
    
    return epochs.sort((a, b) => a.block - b.block)
  }

  const epochData = getEpochData()

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow-sm p-6 border border-gray-200">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
          <div>
            <div className="text-sm font-medium text-gray-500 mb-1">Current Block</div>
            <div className="text-2xl font-bold text-gray-900">
              {data.current_block.height.toLocaleString()}
            </div>
            <div className="text-xs text-gray-500 mt-1">
              {new Date(data.current_block.timestamp).toLocaleString()}
            </div>
          </div>
          <div>
            <div className="text-sm font-medium text-gray-500 mb-1">Average Block Time</div>
            <div className="text-2xl font-bold text-gray-900">
              {data.avg_block_time.toFixed(2)}s
            </div>
          </div>
          <div>
            <div className="text-sm font-medium text-gray-500 mb-1">Timeline Range</div>
            <div className="text-sm font-bold text-gray-900">
              {minBlock.toLocaleString()} - {maxBlock.toLocaleString()}
            </div>
            <div className="text-xs text-gray-500 mt-1">
              ~{Math.round(blocksInTwoMonths / (24 * 3600 / data.avg_block_time))} days
            </div>
          </div>
        </div>

        <div className="relative mt-8">
          <svg
            width="100%"
            height="220"
            className="overflow-visible cursor-pointer"
            onMouseMove={(e) => {
              const rect = e.currentTarget.getBoundingClientRect()
              const x = e.clientX - rect.left
              const ratio = x / rect.width
              const block = Math.round(minBlock + ratio * blockRange)
              setHoveredBlock(block)
              setMousePosition({ x: e.clientX, y: e.clientY })
            }}
            onMouseLeave={() => {
              setHoveredBlock(null)
              setMousePosition(null)
            }}
            onClick={(e) => {
              const rect = e.currentTarget.getBoundingClientRect()
              const x = e.clientX - rect.left
              const ratio = x / rect.width
              const block = Math.round(minBlock + ratio * blockRange)
              handleTimelineClick(block)
            }}
          >
            <line
              x1="0"
              y1="110"
              x2="100%"
              y2="110"
              stroke="#E5E7EB"
              strokeWidth="2"
            />

            {epochData.map((epoch, idx) => {
              const position = ((epoch.block - minBlock) / blockRange) * 100
              if (position < 0 || position > 100) return null
              
              const showLabel = epoch.epochNumber % 3 === 0
              
              return (
                <g
                  key={`epoch-${idx}`}
                  className="cursor-pointer"
                  onMouseEnter={(e) => {
                    e.stopPropagation()
                    setHoveredBlock(epoch.block)
                    setHoveredEpoch(epoch.epochNumber)
                    setMousePosition({ x: e.clientX, y: e.clientY })
                  }}
                  onMouseLeave={() => {
                    setHoveredEpoch(null)
                  }}
                  onClick={(e) => {
                    e.stopPropagation()
                    handleTimelineClick(epoch.block)
                  }}
                >
                  <line
                    x1={`${position}%`}
                    y1="90"
                    x2={`${position}%`}
                    y2="130"
                    stroke="#D1D5DB"
                    strokeWidth="1.5"
                    opacity="0.5"
                  />
                  {showLabel && (
                    <text
                      x={`${position}%`}
                      y="145"
                      textAnchor="middle"
                      className="text-xs fill-gray-500"
                      style={{ fontSize: '10px' }}
                    >
                      E{epoch.epochNumber}
                    </text>
                  )}
                </g>
              )
            })}

            <line
              x1={`${((data.current_block.height - minBlock) / blockRange) * 100}%`}
              y1="70"
              x2={`${((data.current_block.height - minBlock) / blockRange) * 100}%`}
              y2="150"
              stroke="#111827"
              strokeWidth="3"
            />
            <text
              x={`${((data.current_block.height - minBlock) / blockRange) * 100}%`}
              y="170"
              textAnchor="middle"
              className="text-sm fill-gray-900 font-semibold"
            >
              Current
            </text>

            {data.events.map((event, idx) => {
              const position = ((event.block_height - minBlock) / blockRange) * 100
              if (position < 0 || position > 100) return null
              
              const isPast = event.occurred
              const color = isPast ? '#6B7280' : '#3B82F6'
              
              return (
                <g
                  key={idx}
                  className="cursor-pointer transition-all"
                  onClick={(e) => {
                    e.stopPropagation()
                    handleTimelineClick(event.block_height)
                  }}
                >
                  <line
                    x1={`${position}%`}
                    y1="50"
                    x2={`${position}%`}
                    y2="170"
                    stroke={color}
                    strokeWidth="3"
                    strokeDasharray="4 2"
                  />
                  <circle
                    cx={`${position}%`}
                    cy="110"
                    r="6"
                    fill={color}
                  />
                  <text
                    x={`${position}%`}
                    y="40"
                    textAnchor="middle"
                    className="text-xs font-semibold"
                    fill={color}
                  >
                    {event.description}
                  </text>
                  <text
                    x={`${position}%`}
                    y="190"
                    textAnchor="middle"
                    className="text-xs"
                    fill={color}
                  >
                    {event.block_height.toLocaleString()}
                  </text>
                </g>
              )
            })}

            {hoveredBlock !== null && (
              <line
                x1={`${((hoveredBlock - minBlock) / blockRange) * 100}%`}
                y1="70"
                x2={`${((hoveredBlock - minBlock) / blockRange) * 100}%`}
                y2="150"
                stroke="#F59E0B"
                strokeWidth="2"
                opacity="0.5"
              />
            )}
          </svg>
        </div>

        {hoveredBlock !== null && mousePosition && (
          <div
            className="fixed z-50 bg-gray-900 text-white px-4 py-3 rounded-lg shadow-lg text-sm pointer-events-none"
            style={{
              left: mousePosition.x + 10,
              top: mousePosition.y - 60,
            }}
          >
            {hoveredEpoch !== null ? (
              <>
                <div className="font-semibold">Epoch {hoveredEpoch} Start</div>
                <div className="text-xs text-gray-400 mt-1">Block {hoveredBlock.toLocaleString()}</div>
                <div className="text-xs text-gray-300 mt-1">
                  {calculateBlockTime(hoveredBlock)}
                </div>
              </>
            ) : (
              <>
                <div className="font-semibold">Block {hoveredBlock.toLocaleString()}</div>
                <div className="text-xs text-gray-300 mt-1">
                  {calculateBlockTime(hoveredBlock)}
                </div>
              </>
            )}
          </div>
        )}
      </div>

      <div className="bg-white rounded-lg shadow-sm p-6 border border-gray-200">
        <h2 className="text-xl font-bold text-gray-900 mb-4">Network Events</h2>
        
        {data.events.length === 0 ? (
          <p className="text-gray-500">No events scheduled</p>
        ) : (
          <div className="space-y-3">
            {data.events.map((event, index) => {
              const eventTime = calculateBlockTime(event.block_height)
              const isPast = event.occurred

              return (
                <div
                  key={index}
                  className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                    isPast
                      ? 'bg-gray-50 border-gray-300 hover:border-gray-400'
                      : 'bg-blue-50 border-blue-300 hover:border-blue-400'
                  }`}
                  onClick={() => handleTimelineClick(event.block_height)}
                >
                  <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="font-bold text-gray-900">{event.description}</span>
                        <span
                          className={`px-2 py-0.5 text-xs font-semibold rounded ${
                            isPast
                              ? 'bg-gray-200 text-gray-700'
                              : 'bg-blue-200 text-blue-700'
                          }`}
                        >
                          {isPast ? 'PAST' : 'FUTURE'}
                        </span>
                      </div>
                      <div className="text-sm text-gray-600">
                        Block: {event.block_height.toLocaleString()}
                      </div>
                    </div>
                    <div className="text-sm text-gray-600 md:text-right">
                      {eventTime}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
