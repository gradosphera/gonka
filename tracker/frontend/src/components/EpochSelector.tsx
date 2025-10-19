interface EpochSelectorProps {
  currentEpochId: number
  selectedEpochId: number | null
  onSelectEpoch: (epochId: number | null) => void
  disabled: boolean
}

export function EpochSelector({
  currentEpochId,
  selectedEpochId,
  onSelectEpoch,
  disabled
}: EpochSelectorProps) {
  const epochOptions = []
  for (let i = Math.max(1, currentEpochId - 10); i <= currentEpochId; i++) {
    epochOptions.push(i)
  }

  return (
    <div className="flex items-center gap-2">
      <label htmlFor="epoch-select" className="text-sm font-medium text-gray-700 whitespace-nowrap">
        Epoch:
      </label>
      <select
        id="epoch-select"
        value={selectedEpochId || currentEpochId}
        onChange={(e) => {
          const value = e.target.value
          const epochId = parseInt(value)
          onSelectEpoch(epochId === currentEpochId ? null : epochId)
        }}
        disabled={disabled}
        className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-gray-500 focus:border-gray-500 disabled:bg-gray-100 disabled:cursor-not-allowed text-gray-900 bg-white"
      >
        {epochOptions.reverse().map((epoch) => (
          <option key={epoch} value={epoch}>
            Epoch {epoch}{epoch === currentEpochId ? ' (Current)' : ''}
          </option>
        ))}
      </select>
    </div>
  )
}

