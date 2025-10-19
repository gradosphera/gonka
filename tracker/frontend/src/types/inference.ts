export interface CurrentEpochStats {
  inference_count: string;
  missed_requests: string;
  earned_coins: string;
  rewarded_coins: string;
  burned_coins: string;
  validated_inferences: string;
  invalidated_inferences: string;
}

export interface Participant {
  index: string;
  address: string;
  weight: number;
  inference_url?: string;
  status?: string;
  models: string[];
  current_epoch_stats: CurrentEpochStats;
  missed_rate: number;
  invalidation_rate: number;
}

export interface InferenceResponse {
  epoch_id: number;
  height: number;
  participants: Participant[];
  cached_at?: string;
  is_current: boolean;
}

