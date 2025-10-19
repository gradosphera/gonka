# Task 4: Inference Statistics Dashboard Frontend

## Task
Create React dashboard to display inference statistics with auto-refresh and visual highlighting for participants with high missed rates.

## Result
Working frontend dashboard accessible via navigation with:
- Default view: current epoch statistics (auto-refreshes every 30s)
- Epoch selector dropdown for viewing historical data
- Table showing all participants with their stats
- Red highlighting for participants with missed_rate > 10%
- Loading and error states
- Responsive design

## Structure
```
frontend/src/
├── components/
│   ├── InferenceDashboard.tsx    # Main dashboard component
│   ├── ParticipantTable.tsx      # Table with stats
│   ├── EpochSelector.tsx         # Dropdown for epoch selection
│   └── StatCard.tsx              # Summary cards (optional)
├── types/
│   └── inference.ts              # TypeScript types
└── App.tsx                       # Updated with navigation
```

## Approach

### Data Fetching
- Fetch from `/api/v1/inference/current` for current epoch
- Fetch from `/api/v1/inference/epochs/{epoch_id}` for historical
- Auto-refresh every 30 seconds for current epoch only
- Manual refresh button available

### Table Columns
1. Index (participant address/ID)
2. Weight
3. Inference Count
4. Missed Requests
5. Validated Inferences
6. Invalidated Inferences
7. Missed Rate (%)

### Visual Design
- Highlight entire row in red if missed_rate > 0.10
- Use lighter red shade for 0.05 < missed_rate <= 0.10 (optional)
- Sort by missed_rate descending by default
- Alternative sort by weight, index

### Navigation
- Add link/button to switch between pages
- Current pages: Hello (demo), Inference Stats (new)
- Simple top navigation bar

### State Management
- useState for data, loading, error states
- useEffect for fetching and auto-refresh
- Clear interval when component unmounts

### Error Handling
- Display error message if API fails
- Retry mechanism for failed requests
- Show cached timestamp if available

## TypeScript Types
```typescript
interface CurrentEpochStats {
  inference_count: string;
  missed_requests: string;
  validated_inferences: string;
  invalidated_inferences: string;
}

interface Participant {
  index: string;
  address: string;
  weight: number;
  current_epoch_stats: CurrentEpochStats;
  missed_rate: number;
}

interface InferenceResponse {
  epoch_id: number;
  height: number;
  participants: Participant[];
  cached_at?: string;
  is_current: boolean;
}
```

## UI Components

### InferenceDashboard
- Top section: Epoch info, refresh button, epoch selector
- Middle section: Summary statistics (total participants, avg missed rate)
- Bottom section: Participant table

### ParticipantTable
- Sortable columns
- Row highlighting based on missed_rate
- Responsive design for mobile

### EpochSelector
- Dropdown with current and historical epochs
- Disabled when loading
- Updates table data on selection

## User Experience
- Loading spinner while fetching data
- Skeleton table during initial load
- Smooth transitions between epochs
- Clear indication of auto-refresh countdown
- Timestamp of last update

## Testing Considerations
- Test with mock data
- Test auto-refresh behavior
- Test epoch switching
- Test error states
- Test highlighting logic

