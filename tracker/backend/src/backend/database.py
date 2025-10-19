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
            
            await db.commit()
            logger.info(f"Database initialized at {self.db_path}")
    
    async def save_stats(
        self,
        epoch_id: int,
        height: int,
        participant_index: str,
        stats: Dict[str, Any]
    ):
        cached_at = datetime.utcnow().isoformat()
        stats_json = json.dumps(stats)
        
        async with aiosqlite.connect(self.db_path) as db:
            await db.execute("""
                INSERT OR REPLACE INTO inference_stats 
                (epoch_id, height, participant_index, stats_json, cached_at)
                VALUES (?, ?, ?, ?, ?)
            """, (epoch_id, height, participant_index, stats_json, cached_at))
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
                stats_json = json.dumps(stats)
                
                await db.execute("""
                    INSERT OR REPLACE INTO inference_stats 
                    (epoch_id, height, participant_index, stats_json, cached_at)
                    VALUES (?, ?, ?, ?, ?)
                """, (epoch_id, height, participant_index, stats_json, cached_at))
            
            await db.commit()
            logger.info(f"Saved {len(participants_stats)} stats for epoch {epoch_id} at height {height}")
    
    async def get_stats(self, epoch_id: int, height: Optional[int] = None) -> Optional[List[Dict[str, Any]]]:
        async with aiosqlite.connect(self.db_path) as db:
            db.row_factory = aiosqlite.Row
            
            if height is not None:
                query = """
                    SELECT participant_index, stats_json, height, cached_at
                    FROM inference_stats
                    WHERE epoch_id = ? AND height = ?
                """
                params = (epoch_id, height)
            else:
                query = """
                    SELECT participant_index, stats_json, height, cached_at
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

