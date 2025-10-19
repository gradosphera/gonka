import logging
from typing import Optional
from datetime import datetime
from backend.client import GonkaClient
from backend.database import CacheDB
from backend.models import (
    ParticipantStats,
    CurrentEpochStats,
    InferenceResponse
)

logger = logging.getLogger(__name__)


class InferenceService:
    def __init__(self, client: GonkaClient, cache_db: CacheDB):
        self.client = client
        self.cache_db = cache_db
        self.current_epoch_id: Optional[int] = None
        self.current_epoch_data: Optional[InferenceResponse] = None
        self.last_fetch_time: Optional[float] = None
    
    async def get_canonical_height(self, epoch_id: int, requested_height: Optional[int] = None) -> int:
        epoch_data = await self.client.get_epoch_participants(epoch_id)
        effective_height = epoch_data["active_participants"]["effective_block_height"]
        
        next_epoch_data = await self.client.get_epoch_participants(epoch_id + 1)
        next_effective_height = next_epoch_data["active_participants"]["effective_block_height"]
        canonical_height = next_effective_height - 10
        
        if requested_height is None:
            return canonical_height
        
        if requested_height < effective_height:
            raise ValueError(
                f"Height {requested_height} is before epoch {epoch_id} start (effective height: {effective_height}). "
                f"No data exists for this epoch at this height."
            )
        
        if requested_height >= next_effective_height:
            logger.info(f"Height {requested_height} is after epoch {epoch_id} end (next epoch starts at {next_effective_height}). "
                      f"Clamping to canonical height {canonical_height}")
            return canonical_height
        
        return requested_height
    
    async def get_current_epoch_stats(self, reload: bool = False) -> InferenceResponse:
        import time
        
        current_time = time.time()
        cache_age = (current_time - self.last_fetch_time) if self.last_fetch_time else None
        
        if not reload and self.current_epoch_data and cache_age and cache_age < 30:
            logger.info(f"Returning cached current epoch data (age: {cache_age:.1f}s)")
            return self.current_epoch_data
        
        try:
            logger.info("Fetching fresh current epoch data")
            height = await self.client.get_latest_height()
            epoch_data = await self.client.get_current_epoch_participants()
            
            epoch_id = epoch_data["active_participants"]["epoch_group_id"]
            
            await self._mark_epoch_finished_if_needed(epoch_id, height)
            
            all_participants_data = await self.client.get_all_participants(height=height)
            participants_list = all_participants_data.get("participant", [])
            
            active_indices = {
                p["index"] for p in epoch_data["active_participants"]["participants"]
            }
            
            active_participants = [
                p for p in participants_list if p["index"] in active_indices
            ]
            
            participants_stats = []
            for p in active_participants:
                try:
                    participant = ParticipantStats(
                        index=p["index"],
                        address=p["address"],
                        weight=p["weight"],
                        inference_url=p.get("inference_url"),
                        status=p.get("status"),
                        current_epoch_stats=CurrentEpochStats(**p["current_epoch_stats"])
                    )
                    participants_stats.append(participant)
                except Exception as e:
                    logger.warning(f"Failed to parse participant {p.get('index', 'unknown')}: {e}")
            
            response = InferenceResponse(
                epoch_id=epoch_id,
                height=height,
                participants=participants_stats,
                cached_at=datetime.utcnow().isoformat(),
                is_current=True
            )
            
            await self.cache_db.save_stats_batch(
                epoch_id=epoch_id,
                height=height,
                participants_stats=[p.model_dump() for p in participants_stats]
            )
            
            self.current_epoch_id = epoch_id
            self.current_epoch_data = response
            self.last_fetch_time = current_time
            
            logger.info(f"Fetched current epoch {epoch_id} stats at height {height}: {len(participants_stats)} participants")
            
            return response
            
        except Exception as e:
            logger.error(f"Error fetching current epoch stats: {e}")
            if self.current_epoch_data:
                logger.info("Returning cached current epoch data due to error")
                return self.current_epoch_data
            raise
    
    async def get_historical_epoch_stats(self, epoch_id: int, height: Optional[int] = None) -> InferenceResponse:
        is_finished = await self.cache_db.is_epoch_finished(epoch_id)
        
        try:
            target_height = await self.get_canonical_height(epoch_id, height)
        except Exception as e:
            logger.error(f"Failed to determine target height for epoch {epoch_id}: {e}")
            raise
        
        cached_stats = await self.cache_db.get_stats(epoch_id, height=target_height)
        if cached_stats:
            logger.info(f"Returning cached stats for epoch {epoch_id} at height {target_height}")
            
            participants_stats = []
            for stats_dict in cached_stats:
                try:
                    stats_copy = dict(stats_dict)
                    stats_copy.pop("_cached_at", None)
                    stats_copy.pop("_height", None)
                    
                    participant = ParticipantStats(**stats_copy)
                    participants_stats.append(participant)
                except Exception as e:
                    logger.warning(f"Failed to parse cached participant: {e}")
            
            return InferenceResponse(
                epoch_id=epoch_id,
                height=target_height,
                participants=participants_stats,
                cached_at=cached_stats[0].get("_cached_at"),
                is_current=False
            )
        
        try:
            logger.info(f"Fetching historical epoch {epoch_id} at height {target_height}")
            
            all_participants_data = await self.client.get_all_participants(height=target_height)
            participants_list = all_participants_data.get("participant", [])
            
            epoch_data = await self.client.get_epoch_participants(epoch_id)
            active_indices = {
                p["index"] for p in epoch_data["active_participants"]["participants"]
            }
            
            active_participants = [
                p for p in participants_list if p["index"] in active_indices
            ]
            
            participants_stats = []
            for p in active_participants:
                try:
                    participant = ParticipantStats(
                        index=p["index"],
                        address=p["address"],
                        weight=p["weight"],
                        inference_url=p.get("inference_url"),
                        status=p.get("status"),
                        current_epoch_stats=CurrentEpochStats(**p["current_epoch_stats"])
                    )
                    participants_stats.append(participant)
                except Exception as e:
                    logger.warning(f"Failed to parse participant {p.get('index', 'unknown')}: {e}")
            
            await self.cache_db.save_stats_batch(
                epoch_id=epoch_id,
                height=target_height,
                participants_stats=[p.model_dump() for p in participants_stats]
            )
            
            if height is None and not is_finished:
                await self.cache_db.mark_epoch_finished(epoch_id, target_height)
            
            response = InferenceResponse(
                epoch_id=epoch_id,
                height=target_height,
                participants=participants_stats,
                cached_at=datetime.utcnow().isoformat(),
                is_current=False
            )
            
            logger.info(f"Fetched and cached historical epoch {epoch_id} at height {target_height}: {len(participants_stats)} participants")
            
            return response
            
        except Exception as e:
            logger.error(f"Error fetching historical epoch {epoch_id}: {e}")
            raise
    
    async def _mark_epoch_finished_if_needed(self, current_epoch_id: int, current_height: int):
        if self.current_epoch_id is None:
            return
        
        if current_epoch_id > self.current_epoch_id:
            old_epoch_id = self.current_epoch_id
            is_already_finished = await self.cache_db.is_epoch_finished(old_epoch_id)
            
            if not is_already_finished:
                logger.info(f"Epoch transition detected: {old_epoch_id} -> {current_epoch_id}")
                
                try:
                    await self.get_historical_epoch_stats(old_epoch_id)
                    logger.info(f"Marked epoch {old_epoch_id} as finished and cached final stats")
                except Exception as e:
                    logger.error(f"Failed to mark epoch {old_epoch_id} as finished: {e}")

