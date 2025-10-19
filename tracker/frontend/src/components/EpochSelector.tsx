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
      <label htmlFor="epoch-select" className="text-sm font-medium text-gray-700">
        Epoch:
      </label>
      <select
        id="epoch-select"
        value={selectedEpochId || ''}
        onChange={(e) => {
          const value = e.target.value
          onSelectEpoch(value === '' ? null : parseInt(value))
        }}
        disabled={disabled}
        className="px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:cursor-not-allowed"
      >
        <option value="">Current Epoch ({currentEpochId})</option>
        {epochOptions.reverse().map((epoch) => (
          <option key={epoch} value={epoch}>
            Epoch {epoch}
          </option>
        ))}
      </select>
    </div>
  )
}

