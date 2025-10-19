import { Participant } from '../types/inference'

interface ParticipantTableProps {
  participants: Participant[]
}

export function ParticipantTable({ participants }: ParticipantTableProps) {
  const sortedParticipants = [...participants].sort((a, b) => b.weight - a.weight)

  const truncateAddress = (address: string) => {
    if (address.length <= 16) return address
    return `${address.slice(0, 12)}...${address.slice(-4)}`
  }

  const shouldHighlightRed = (participant: Participant) => {
    return participant.missed_rate > 0.10 || participant.invalidation_rate > 0.10
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full bg-white border border-gray-300 shadow-sm">
        <thead className="bg-gray-100">
          <tr>
            <th className="px-4 py-3 text-left text-sm font-semibold text-gray-700 border-b">
              Index
            </th>
            <th className="px-4 py-3 text-left text-sm font-semibold text-gray-700 border-b">
              Weight
            </th>
            <th className="px-4 py-3 text-left text-sm font-semibold text-gray-700 border-b">
              Models
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Inferences
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Missed
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Validated
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Invalidated
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Missed Rate
            </th>
            <th className="px-4 py-3 text-right text-sm font-semibold text-gray-700 border-b">
              Invalid Rate
            </th>
          </tr>
        </thead>
        <tbody>
          {sortedParticipants.map((participant) => (
            <tr
              key={participant.index}
              className={`border-b hover:bg-gray-50 ${
                shouldHighlightRed(participant) ? 'bg-red-100' : ''
              }`}
            >
              <td className="px-4 py-3 text-sm text-gray-900" title={participant.index}>
                {truncateAddress(participant.index)}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900">
                {participant.weight.toLocaleString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-600">
                {participant.models.length > 0 ? (
                  <div className="flex flex-wrap gap-1">
                    {participant.models.map((model, idx) => (
                      <span
                        key={idx}
                        className="inline-block px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded"
                      >
                        {model}
                      </span>
                    ))}
                  </div>
                ) : (
                  <span className="text-gray-400">-</span>
                )}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right">
                {parseInt(participant.current_epoch_stats.inference_count).toLocaleString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right">
                {parseInt(participant.current_epoch_stats.missed_requests).toLocaleString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right">
                {parseInt(participant.current_epoch_stats.validated_inferences).toLocaleString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right">
                {parseInt(participant.current_epoch_stats.invalidated_inferences).toLocaleString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right font-medium">
                {(participant.missed_rate * 100).toFixed(2)}%
              </td>
              <td className="px-4 py-3 text-sm text-gray-900 text-right font-medium">
                {(participant.invalidation_rate * 100).toFixed(2)}%
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

