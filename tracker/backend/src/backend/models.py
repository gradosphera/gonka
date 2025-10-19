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
    validator_key: Optional[str] = None
    inference_url: Optional[str] = None
    status: Optional[str] = None
    models: List[str] = []
    current_epoch_stats: CurrentEpochStats
    is_jailed: Optional[bool] = None
    jailed_until: Optional[str] = None
    ready_to_unjail: Optional[bool] = None
    node_healthy: Optional[bool] = None
    node_health_checked_at: Optional[str] = None
    
    @computed_field
    @property
    def missed_rate(self) -> float:
        missed = int(self.current_epoch_stats.missed_requests)
        inferences = int(self.current_epoch_stats.inference_count)
        total = missed + inferences
        
        if total == 0:
            return 0.0
        
        return round(missed / total, 4)
    
    @computed_field
    @property
    def invalidation_rate(self) -> float:
        invalidated = int(self.current_epoch_stats.invalidated_inferences)
        inferences = int(self.current_epoch_stats.inference_count)
        
        if inferences == 0:
            return 0.0
        
        return round(invalidated / inferences, 4)


class InferenceResponse(BaseModel):
    epoch_id: int
    height: int
    participants: List[ParticipantStats]
    cached_at: Optional[str] = None
    is_current: bool = False
    total_assigned_rewards_gnk: Optional[int] = None


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


class RewardInfo(BaseModel):
    epoch_id: int
    assigned_reward_gnk: int
    claimed: bool


class SeedInfo(BaseModel):
    participant: str
    epoch_index: int
    signature: str


class WarmKeyInfo(BaseModel):
    grantee_address: str
    granted_at: str


class HardwareInfo(BaseModel):
    type: str
    count: int


class MLNodeInfo(BaseModel):
    local_id: str
    status: str
    models: List[str]
    hardware: List[HardwareInfo]
    host: str
    port: str


class ParticipantDetailsResponse(BaseModel):
    participant: ParticipantStats
    rewards: List[RewardInfo]
    seed: Optional[SeedInfo]
    warm_keys: List[WarmKeyInfo]
    ml_nodes: List[MLNodeInfo]


class LatestEpochInfo(BaseModel):
    block_height: int
    latest_epoch: dict
    phase: str

