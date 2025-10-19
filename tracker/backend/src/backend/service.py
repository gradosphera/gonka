import logging
from typing import Optional, List, Dict, Any
from datetime import datetime, timezone
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
            
            epoch_participant_data = {
                p["index"]: {
                    "weight": p.get("weight", 0),
                    "models": p.get("models", [])
                }
                for p in epoch_data["active_participants"]["participants"]
            }
            
            active_participants = [
                p for p in participants_list if p["index"] in active_indices
            ]
            
            participants_stats = []
            for p in active_participants:
                try:
                    epoch_data_for_participant = epoch_participant_data.get(p["index"], {})
                    participant = ParticipantStats(
                        index=p["index"],
                        address=p["address"],
                        weight=epoch_data_for_participant.get("weight", 0),
                        inference_url=p.get("inference_url"),
                        status=p.get("status"),
                        models=epoch_data_for_participant.get("models", []),
                        current_epoch_stats=CurrentEpochStats(**p["current_epoch_stats"])
                    )
                    participants_stats.append(participant)
                except Exception as e:
                    logger.warning(f"Failed to parse participant {p.get('index', 'unknown')}: {e}")
            
            active_participants_list = epoch_data["active_participants"]["participants"]
            participants_stats = await self.merge_jail_and_health_data(epoch_id, participants_stats, height, active_participants_list)
            
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
            
            epoch_data = await self.client.get_epoch_participants(epoch_id)
            active_participants_list = epoch_data["active_participants"]["participants"]
            participants_stats = await self.merge_jail_and_health_data(epoch_id, participants_stats, target_height, active_participants_list)
            
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
            
            epoch_participant_data = {
                p["index"]: {
                    "weight": p.get("weight", 0),
                    "models": p.get("models", [])
                }
                for p in epoch_data["active_participants"]["participants"]
            }
            
            active_participants = [
                p for p in participants_list if p["index"] in active_indices
            ]
            
            participants_stats = []
            for p in active_participants:
                try:
                    epoch_data_for_participant = epoch_participant_data.get(p["index"], {})
                    participant = ParticipantStats(
                        index=p["index"],
                        address=p["address"],
                        weight=epoch_data_for_participant.get("weight", 0),
                        inference_url=p.get("inference_url"),
                        status=p.get("status"),
                        models=epoch_data_for_participant.get("models", []),
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
            
            participants_stats = await self.merge_jail_and_health_data(epoch_id, participants_stats, target_height, epoch_data["active_participants"]["participants"])
            
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
    
    async def fetch_and_cache_jail_statuses(self, epoch_id: int, height: int, active_participants: List[Dict[str, Any]]):
        try:
            validators = await self.client.get_all_validators(height=height)
            validators_with_tokens = [v for v in validators if v.get("tokens") and int(v.get("tokens")) > 0]
            
            active_indices = {p["index"] for p in active_participants}
            
            participant_pubkey_map = {p["index"]: p.get("validator_key") for p in active_participants}
            
            jail_statuses = []
            now_utc = datetime.now(timezone.utc)
            
            for validator in validators_with_tokens:
                operator_address = validator.get("operator_address", "")
                
                consensus_pub = (
                    (validator.get("consensus_pubkey") or {}).get("key")
                    or (validator.get("consensus_pubkey") or {}).get("value")
                    or ""
                )
                
                if not consensus_pub:
                    continue
                
                participant_index = None
                for index, pubkey in participant_pubkey_map.items():
                    if pubkey == consensus_pub:
                        participant_index = index
                        break
                
                if not participant_index or participant_index not in active_indices:
                    continue
                
                is_jailed = bool(validator.get("jailed"))
                valcons_addr = self.client.pubkey_to_valcons(consensus_pub)
                
                jailed_until = None
                ready_to_unjail = False
                
                if is_jailed:
                    signing_info = await self.client.get_signing_info(valcons_addr, height=height)
                    if signing_info:
                        jailed_until_str = signing_info.get("jailed_until")
                        if jailed_until_str and "1970-01-01" not in jailed_until_str:
                            jailed_until = jailed_until_str
                            try:
                                jailed_until_dt = datetime.fromisoformat(jailed_until_str.replace("Z", "")).replace(tzinfo=timezone.utc)
                                ready_to_unjail = now_utc > jailed_until_dt
                            except Exception:
                                pass
                
                jail_statuses.append({
                    "participant_index": participant_index,
                    "is_jailed": is_jailed,
                    "jailed_until": jailed_until,
                    "ready_to_unjail": ready_to_unjail,
                    "valcons_address": valcons_addr
                })
            
            await self.cache_db.save_jail_status_batch(epoch_id, jail_statuses)
            logger.info(f"Cached jail statuses for {len(jail_statuses)} participants in epoch {epoch_id}")
            
        except Exception as e:
            logger.error(f"Failed to fetch and cache jail statuses: {e}")
    
    async def fetch_and_cache_node_health(self, active_participants: List[Dict[str, Any]]):
        try:
            health_statuses = []
            
            for participant in active_participants:
                participant_index = participant.get("index")
                inference_url = participant.get("inference_url")
                
                if not participant_index:
                    continue
                
                health_result = await self.client.check_node_health(inference_url)
                
                health_statuses.append({
                    "participant_index": participant_index,
                    "is_healthy": health_result["is_healthy"],
                    "error_message": health_result["error_message"],
                    "response_time_ms": health_result["response_time_ms"]
                })
            
            await self.cache_db.save_node_health_batch(health_statuses)
            logger.info(f"Cached health statuses for {len(health_statuses)} participants")
            
        except Exception as e:
            logger.error(f"Failed to fetch and cache node health: {e}")
    
    async def merge_jail_and_health_data(self, epoch_id: int, participants: List[ParticipantStats], height: int, active_participants: List[Dict[str, Any]]) -> List[ParticipantStats]:
        try:
            jail_statuses_list = await self.cache_db.get_jail_status(epoch_id)
            jail_map = {}
            if jail_statuses_list:
                jail_map = {j["participant_index"]: j for j in jail_statuses_list}
            else:
                logger.info(f"No cached jail statuses for epoch {epoch_id}, fetching inline")
                await self.fetch_and_cache_jail_statuses(epoch_id, height, active_participants)
                jail_statuses_list = await self.cache_db.get_jail_status(epoch_id)
                if jail_statuses_list:
                    jail_map = {j["participant_index"]: j for j in jail_statuses_list}
            
            health_statuses_list = await self.cache_db.get_node_health()
            health_map = {}
            if health_statuses_list:
                health_map = {h["participant_index"]: h for h in health_statuses_list}
            else:
                logger.info("No cached health statuses, fetching inline")
                await self.fetch_and_cache_node_health(active_participants)
                health_statuses_list = await self.cache_db.get_node_health()
                if health_statuses_list:
                    health_map = {h["participant_index"]: h for h in health_statuses_list}
            
            for participant in participants:
                jail_info = jail_map.get(participant.index)
                if jail_info:
                    participant.is_jailed = jail_info["is_jailed"]
                    participant.jailed_until = jail_info["jailed_until"]
                    participant.ready_to_unjail = jail_info["ready_to_unjail"]
                
                health_info = health_map.get(participant.index)
                if health_info:
                    participant.node_healthy = health_info["is_healthy"]
                    participant.node_health_checked_at = health_info["last_check"]
            
            return participants
            
        except Exception as e:
            logger.error(f"Failed to merge jail and health data: {e}")
            return participants

