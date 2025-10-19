import pytest
import pytest_asyncio
import tempfile
import os
from backend.database import CacheDB


@pytest_asyncio.fixture
async def db():
    with tempfile.NamedTemporaryFile(delete=False, suffix=".db") as f:
        db_path = f.name
    
    cache_db = CacheDB(db_path)
    await cache_db.initialize()
    
    yield cache_db
    
    if os.path.exists(db_path):
        os.unlink(db_path)


@pytest.mark.asyncio
async def test_database_initialization(db):
    assert os.path.exists(db.db_path)


@pytest.mark.asyncio
async def test_save_and_get_stats(db):
    stats = {
        "index": "participant_1",
        "address": "gonka1abc...",
        "inference_count": "10",
        "missed_requests": "2",
        "validated_inferences": "8",
        "invalidated_inferences": "0"
    }
    
    await db.save_stats(epoch_id=1, height=1000, participant_index="participant_1", stats=stats)
    
    result = await db.get_stats(epoch_id=1)
    assert result is not None
    assert len(result) == 1
    assert result[0]["index"] == "participant_1"
    assert result[0]["inference_count"] == "10"
    assert "_cached_at" in result[0]
    assert "_height" in result[0]


@pytest.mark.asyncio
async def test_save_stats_batch(db):
    participants_stats = [
        {
            "index": "participant_1",
            "address": "gonka1abc...",
            "inference_count": "10",
            "missed_requests": "2"
        },
        {
            "index": "participant_2",
            "address": "gonka1def...",
            "inference_count": "15",
            "missed_requests": "1"
        }
    ]
    
    await db.save_stats_batch(epoch_id=2, height=2000, participants_stats=participants_stats)
    
    result = await db.get_stats(epoch_id=2)
    assert result is not None
    assert len(result) == 2


@pytest.mark.asyncio
async def test_has_stats_for_epoch(db):
    assert not await db.has_stats_for_epoch(epoch_id=3)
    
    await db.save_stats(
        epoch_id=3,
        height=3000,
        participant_index="participant_1",
        stats={"index": "participant_1", "inference_count": "5"}
    )
    
    assert await db.has_stats_for_epoch(epoch_id=3)


@pytest.mark.asyncio
async def test_mark_and_check_epoch_finished(db):
    assert not await db.is_epoch_finished(epoch_id=4)
    
    await db.mark_epoch_finished(epoch_id=4, finish_height=4000)
    
    assert await db.is_epoch_finished(epoch_id=4)
    
    finish_height = await db.get_epoch_finish_height(epoch_id=4)
    assert finish_height == 4000


@pytest.mark.asyncio
async def test_epoch_stats_immutability(db):
    stats_v1 = {
        "index": "participant_1",
        "inference_count": "10"
    }
    
    await db.save_stats(epoch_id=5, height=5000, participant_index="participant_1", stats=stats_v1)
    await db.mark_epoch_finished(epoch_id=5, finish_height=5000)
    
    result_v1 = await db.get_stats(epoch_id=5, height=5000)
    assert result_v1[0]["inference_count"] == "10"
    
    stats_v2 = {
        "index": "participant_1",
        "inference_count": "20"
    }
    await db.save_stats(epoch_id=5, height=5100, participant_index="participant_1", stats=stats_v2)
    
    result_v2_at_5100 = await db.get_stats(epoch_id=5, height=5100)
    assert result_v2_at_5100[0]["inference_count"] == "20"
    
    result_v1_still_at_5000 = await db.get_stats(epoch_id=5, height=5000)
    assert result_v1_still_at_5000[0]["inference_count"] == "10"


@pytest.mark.asyncio
async def test_clear_epoch_stats(db):
    await db.save_stats(
        epoch_id=6,
        height=6000,
        participant_index="participant_1",
        stats={"index": "participant_1"}
    )
    await db.mark_epoch_finished(epoch_id=6, finish_height=6000)
    
    assert await db.has_stats_for_epoch(epoch_id=6)
    assert await db.is_epoch_finished(epoch_id=6)
    
    await db.clear_epoch_stats(epoch_id=6)
    
    assert not await db.has_stats_for_epoch(epoch_id=6)
    assert not await db.is_epoch_finished(epoch_id=6)


@pytest.mark.asyncio
async def test_multiple_participants_same_epoch(db):
    participants = [
        {"index": f"participant_{i}", "inference_count": f"{i * 10}"}
        for i in range(1, 11)
    ]
    
    await db.save_stats_batch(epoch_id=7, height=7000, participants_stats=participants)
    
    result = await db.get_stats(epoch_id=7)
    assert len(result) == 10
    
    indices = [p["index"] for p in result]
    assert "participant_1" in indices
    assert "participant_10" in indices


@pytest.mark.asyncio
async def test_save_and_retrieve_models(db):
    stats = {
        "index": "participant_1",
        "address": "gonka1abc...",
        "models": ["Llama-3.1-8B", "Qwen2.5-7B"],
        "inference_count": "10",
        "missed_requests": "2"
    }
    
    await db.save_stats(epoch_id=8, height=8000, participant_index="participant_1", stats=stats)
    
    result = await db.get_stats(epoch_id=8)
    assert result is not None
    assert len(result) == 1
    assert result[0]["index"] == "participant_1"
    assert result[0]["models"] == ["Llama-3.1-8B", "Qwen2.5-7B"]


@pytest.mark.asyncio
async def test_save_stats_with_empty_models(db):
    stats = {
        "index": "participant_2",
        "address": "gonka1def...",
        "models": [],
        "inference_count": "5"
    }
    
    await db.save_stats(epoch_id=9, height=9000, participant_index="participant_2", stats=stats)
    
    result = await db.get_stats(epoch_id=9)
    assert result is not None
    assert result[0]["models"] == []

