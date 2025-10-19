import aiosqlite
import json
from typing import List, Dict, Any, Optional
from datetime import datetime
from pathlib import Path
import logging

logger = logging.getLogger(__name__)


class CacheDB:
    def __init__(self, db_path: str = "cache.db"):
        self.db_path = db_path
        
    async def initialize(self):
        async with aiosqlite.connect(self.db_path) as db:
            await db.execute("""
                CREATE TABLE IF NOT EXISTS inference_stats (
                    epoch_id INTEGER NOT NULL,
                    height INTEGER NOT NULL,
                    participant_index TEXT NOT NULL,
                    stats_json TEXT NOT NULL,
                    seed_signature TEXT,
                    cached_at TEXT NOT NULL,
                    PRIMARY KEY (epoch_id, height, participant_index)
                )
            """)
            
            await db.execute("""
                CREATE INDEX IF NOT EXISTS idx_epoch_height 
                ON inference_stats(epoch_id, height)
            """)
            
            await db.execute("""
                CREATE TABLE IF NOT EXISTS epoch_status (
                    epoch_id INTEGER PRIMARY KEY,
                    is_finished BOOLEAN NOT NULL,
                    finish_height INTEGER,
                    marked_at TEXT NOT NULL
                )
            """)
            
            await db.execute("""
                CREATE TABLE IF NOT EXISTS jail_status (
                    epoch_id INTEGER NOT NULL,
                    participant_index TEXT NOT NULL,
                    is_jailed BOOLEAN NOT NULL,
                    jailed_until TEXT,
                    ready_to_unjail BOOLEAN,
                    valcons_address TEXT,
                    recorded_at TEXT NOT NULL,
                    PRIMARY KEY (epoch_id, participant_index)
                )
            """)
            
            await db.execute("""
                CREATE INDEX IF NOT EXISTS idx_participant_jail 
                ON jail_status(participant_index)
            """)
            
            await db.execute("""
                CREATE TABLE IF NOT EXISTS node_health (
                    participant_index TEXT NOT NULL,
                    is_healthy BOOLEAN NOT NULL,
                    last_check TEXT NOT NULL,
                    error_message TEXT,
                    response_time_ms INTEGER,
                    PRIMARY KEY (participant_index)
                )
            """)
            
            await db.execute("""
                CREATE TABLE IF NOT EXISTS participant_rewards (
                    epoch_id INTEGER NOT NULL,
                    participant_id TEXT NOT NULL,
                    rewarded_coins TEXT NOT NULL,
                    claimed INTEGER NOT NULL,
                    last_updated TEXT NOT NULL,
                    PRIMARY KEY (epoch_id, participant_id)
                )
            """)
            
            await db.execute("""
                CREATE INDEX IF NOT EXISTS idx_participant_rewards
                ON participant_rewards(participant_id)
            """)
            
            await db.commit()
            logger.info(f"Database initialized at {self.db_path}")
    
    async def save_stats(
        self,
        epoch_id: int,
        height: int,
        participant_index: str,
        stats: Dict[str, Any],
        seed_signature: Optional[str] = None
    ):
        cached_at = datetime.utcnow().isoformat()
        stats_json = json.dumps(stats)
        
        async with aiosqlite.connect(self.db_path) as db:
            await db.execute("""
                INSERT OR REPLACE INTO inference_stats 
                (epoch_id, height, participant_index, stats_json, seed_signature, cached_at)
                VALUES (?, ?, ?, ?, ?, ?)
            """, (epoch_id, height, participant_index, stats_json, seed_signature, cached_at))
            await db.commit()
    
    async def save_stats_batch(
        self,
        epoch_id: int,
        height: int,
        participants_stats: List[Dict[str, Any]]
    ):
        cached_at = datetime.utcnow().isoformat()
        
        async with aiosqlite.connect(self.db_path) as db:
            for stats in participants_stats:
                participant_index = stats.get("index")
                seed_signature = stats.get("seed_signature")
                stats_json = json.dumps(stats)
                
                await db.execute("""
                    INSERT OR REPLACE INTO inference_stats 
                    (epoch_id, height, participant_index, stats_json, seed_signature, cached_at)
                    VALUES (?, ?, ?, ?, ?, ?)
                """, (epoch_id, height, participant_index, stats_json, seed_signature, cached_at))
            
            await db.commit()
            logger.info(f"Saved {len(participants_stats)} stats for epoch {epoch_id} at height {height}")
    
    async def get_stats(self, epoch_id: int, height: Optional[int] = None) -> Optional[List[Dict[str, Any]]]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            if height is not None:
                query = """
                    SELECT participant_index, stats_json, seed_signature, height, cached_at
                    FROM inference_stats
                    WHERE epoch_id = ? AND height = ?
                """
                params = (epoch_id, height)
            else:
                query = """
                    SELECT participant_index, stats_json, seed_signature, height, cached_at
                    FROM inference_stats
                    WHERE epoch_id = ?
                """
                params = (epoch_id,)
            
            async with db.execute(query, params) as cursor:
                rows = await cursor.fetchall()
                
                if not rows:
                    return None
                
                results = []
                for row in rows:
                    stats = json.loads(row["stats_json"])
                    stats["_cached_at"] = row["cached_at"]
                    stats["_height"] = row["height"]
                    stats["_seed_signature"] = row["seed_signature"]
                    results.append(stats)
                
                return results
    
    async def has_stats_for_epoch(self, epoch_id: int, height: Optional[int] = None) -> bool:
        async with aiosqlite.connect(self.db_path) as db:
            if height is not None:
                query = "SELECT COUNT(*) as count FROM inference_stats WHERE epoch_id = ? AND height = ?"
                params = (epoch_id, height)
            else:
                query = "SELECT COUNT(*) as count FROM inference_stats WHERE epoch_id = ?"
                params = (epoch_id,)
            
            async with db.execute(query, params) as cursor:
                row = await cursor.fetchone()
                return row[0] > 0
    
    async def mark_epoch_finished(self, epoch_id: int, finish_height: int):
        marked_at = datetime.utcnow().isoformat()
        
        async with aiosqlite.connect(self.db_path) as db:
            await db.execute("""
                INSERT OR REPLACE INTO epoch_status 
                (epoch_id, is_finished, finish_height, marked_at)
                VALUES (?, ?, ?, ?)
            """, (epoch_id, True, finish_height, marked_at))
            await db.commit()
            logger.info(f"Marked epoch {epoch_id} as finished at height {finish_height}")
    
    async def is_epoch_finished(self, epoch_id: int) -> bool:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            async with db.execute("""
                SELECT is_finished FROM epoch_status WHERE epoch_id = ?
            """, (epoch_id,)) as cursor:
                row = await cursor.fetchone()
                return row["is_finished"] if row else False
    
    async def get_epoch_finish_height(self, epoch_id: int) -> Optional[int]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            async with db.execute("""
                SELECT finish_height FROM epoch_status WHERE epoch_id = ?
            """, (epoch_id,)) as cursor:
                row = await cursor.fetchone()
                return row["finish_height"] if row else None
    
    async def clear_epoch_stats(self, epoch_id: int):
        async with aiosqlite.connect(self.db_path) as db:
            await db.execute("DELETE FROM inference_stats WHERE epoch_id = ?", (epoch_id,))
            await db.execute("DELETE FROM epoch_status WHERE epoch_id = ?", (epoch_id,))
            await db.commit()
    
    async def save_jail_status_batch(
        self,
        epoch_id: int,
        jail_statuses: List[Dict[str, Any]]
    ):
        recorded_at = datetime.utcnow().isoformat()
        
        async with aiosqlite.connect(self.db_path) as db:
            for status in jail_statuses:
                await db.execute("""
                    INSERT OR REPLACE INTO jail_status 
                    (epoch_id, participant_index, is_jailed, jailed_until, ready_to_unjail, valcons_address, recorded_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                """, (
                    epoch_id,
                    status.get("participant_index"),
                    status.get("is_jailed", False),
                    status.get("jailed_until"),
                    status.get("ready_to_unjail", False),
                    status.get("valcons_address"),
                    recorded_at
                ))
            
            await db.commit()
            logger.info(f"Saved {len(jail_statuses)} jail statuses for epoch {epoch_id}")
    
    async def get_jail_status(self, epoch_id: int, participant_index: Optional[str] = None) -> Optional[List[Dict[str, Any]]]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            if participant_index:
                query = """
                    SELECT * FROM jail_status
                    WHERE epoch_id = ? AND participant_index = ?
                """
                params = (epoch_id, participant_index)
            else:
                query = """
                    SELECT * FROM jail_status
                    WHERE epoch_id = ?
                """
                params = (epoch_id,)
            
            async with db.execute(query, params) as cursor:
                rows = await cursor.fetchall()
                
                if not rows:
                    return None
                
                results = []
                for row in rows:
                    results.append({
                        "epoch_id": row["epoch_id"],
                        "participant_index": row["participant_index"],
                        "is_jailed": bool(row["is_jailed"]),
                        "jailed_until": row["jailed_until"],
                        "ready_to_unjail": bool(row["ready_to_unjail"]) if row["ready_to_unjail"] is not None else None,
                        "valcons_address": row["valcons_address"],
                        "recorded_at": row["recorded_at"]
                    })
                
                return results
    
    async def save_node_health_batch(
        self,
        health_statuses: List[Dict[str, Any]]
    ):
        last_check = datetime.utcnow().isoformat()
        
        async with aiosqlite.connect(self.db_path) as db:
            for status in health_statuses:
                await db.execute("""
                    INSERT OR REPLACE INTO node_health 
                    (participant_index, is_healthy, last_check, error_message, response_time_ms)
                    VALUES (?, ?, ?, ?, ?)
                """, (
                    status.get("participant_index"),
                    status.get("is_healthy", False),
                    last_check,
                    status.get("error_message"),
                    status.get("response_time_ms")
                ))
            
            await db.commit()
            logger.info(f"Saved {len(health_statuses)} node health statuses")
    
    async def get_node_health(self, participant_index: Optional[str] = None) -> Optional[List[Dict[str, Any]]]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            if participant_index:
                query = "SELECT * FROM node_health WHERE participant_index = ?"
                params = (participant_index,)
            else:
                query = "SELECT * FROM node_health"
                params = ()
            
            async with db.execute(query, params) as cursor:
                rows = await cursor.fetchall()
                
                if not rows:
                    return None
                
                results = []
                for row in rows:
                    results.append({
                        "participant_index": row["participant_index"],
                        "is_healthy": bool(row["is_healthy"]),
                        "last_check": row["last_check"],
                        "error_message": row["error_message"],
                        "response_time_ms": row["response_time_ms"]
                    })
                
                return results
    
    async def save_reward_batch(
        self,
        rewards: List[Dict[str, Any]]
    ):
        last_updated = datetime.utcnow().isoformat()
        
        async with aiosqlite.connect(self.db_path) as db:
            for reward in rewards:
                await db.execute("""
                    INSERT OR REPLACE INTO participant_rewards 
                    (epoch_id, participant_id, rewarded_coins, claimed, last_updated)
                    VALUES (?, ?, ?, ?, ?)
                """, (
                    reward.get("epoch_id"),
                    reward.get("participant_id"),
                    reward.get("rewarded_coins", "0"),
                    1 if reward.get("claimed") else 0,
                    last_updated
                ))
            
            await db.commit()
            logger.info(f"Saved {len(rewards)} rewards")
    
    async def get_reward(self, epoch_id: int, participant_id: str) -> Optional[Dict[str, Any]]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            async with db.execute("""
                SELECT * FROM participant_rewards
                WHERE epoch_id = ? AND participant_id = ?
            """, (epoch_id, participant_id)) as cursor:
                row = await cursor.fetchone()
                
                if not row:
                    return None
                
                return {
                    "epoch_id": row["epoch_id"],
                    "participant_id": row["participant_id"],
                    "rewarded_coins": row["rewarded_coins"],
                    "claimed": bool(row["claimed"]),
                    "last_updated": row["last_updated"]
                }
    
    async def get_rewards_for_participant(
        self,
        participant_id: str,
        epoch_ids: List[int]
    ) -> List[Dict[str, Any]]:
        if not epoch_ids:
            return []
        
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            placeholders = ",".join("?" * len(epoch_ids))
            query = f"""
                SELECT * FROM participant_rewards
                WHERE participant_id = ? AND epoch_id IN ({placeholders})
                ORDER BY epoch_id DESC
            """
            params = [participant_id] + epoch_ids
            
            async with db.execute(query, params) as cursor:
                rows = await cursor.fetchall()
                
                results = []
                for row in rows:
                    results.append({
                        "epoch_id": row["epoch_id"],
                        "participant_id": row["participant_id"],
                        "rewarded_coins": row["rewarded_coins"],
                        "claimed": bool(row["claimed"]),
                        "last_updated": row["last_updated"]
                    })
                
                return results

