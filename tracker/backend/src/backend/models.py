from pydantic import BaseModel, Field, computed_field
from typing import Optional, List
from datetime import datetime


class CurrentEpochStats(BaseModel):
    inference_count: str
    missed_requests: str
    earned_coins: str
    rewarded_coins: str
    burned_coins: str
    validated_inferences: str
    invalidated_inferences: str


class ParticipantStats(BaseModel):
    index: str
    address: str
    weight: int
    inference_url: Optional[str] = None
    status: Optional[str] = None
    current_epoch_stats: CurrentEpochStats
    
    @computed_field
    @property
    def missed_rate(self) -> float:
        missed = int(self.current_epoch_stats.missed_requests)
        inferences = int(self.current_epoch_stats.inference_count)
        total = missed + inferences
        
        if total == 0:
            return 0.0
        
        return round(missed / total, 4)


class InferenceResponse(BaseModel):
    epoch_id: int
    height: int
    participants: List[ParticipantStats]
    cached_at: Optional[str] = None
    is_current: bool = False


class EpochParticipant(BaseModel):
    index: str
    validator_key: str
    weight: int
    inference_url: str
    models: List[str]


class EpochInfo(BaseModel):
    epoch_group_id: int
    poc_start_block_height: int
    effective_block_height: int
    created_at_block_height: int
    participants: List[EpochParticipant]

