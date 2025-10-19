import pytest
import json
from pathlib import Path
from backend.models import (
    ParticipantStats,
    CurrentEpochStats,
    InferenceResponse,
    EpochInfo
)


def test_current_epoch_stats():
    stats = CurrentEpochStats(
        inference_count="10",
        missed_requests="2",
        earned_coins="100",
        rewarded_coins="95",
        burned_coins="5",
        validated_inferences="8",
        invalidated_inferences="2"
    )
    
    assert stats.inference_count == "10"
    assert stats.missed_requests == "2"


def test_participant_stats_missed_rate():
    stats = ParticipantStats(
        index="participant_1",
        address="gonka1abc...",
        weight=100,
        current_epoch_stats=CurrentEpochStats(
            inference_count="8",
            missed_requests="2",
            earned_coins="0",
            rewarded_coins="0",
            burned_coins="0",
            validated_inferences="8",
            invalidated_inferences="0"
        )
    )
    
    assert stats.missed_rate == 0.2


def test_participant_stats_zero_total():
    stats = ParticipantStats(
        index="participant_2",
        address="gonka1def...",
        weight=50,
        current_epoch_stats=CurrentEpochStats(
            inference_count="0",
            missed_requests="0",
            earned_coins="0",
            rewarded_coins="0",
            burned_coins="0",
            validated_inferences="0",
            invalidated_inferences="0"
        )
    )
    
    assert stats.missed_rate == 0.0


def test_participant_stats_high_missed_rate():
    stats = ParticipantStats(
        index="participant_3",
        address="gonka1ghi...",
        weight=200,
        current_epoch_stats=CurrentEpochStats(
            inference_count="5",
            missed_requests="95",
            earned_coins="0",
            rewarded_coins="0",
            burned_coins="0",
            validated_inferences="5",
            invalidated_inferences="0"
        )
    )
    
    assert stats.missed_rate == 0.95


def test_inference_response():
    participants = [
        ParticipantStats(
            index="participant_1",
            address="gonka1abc...",
            weight=100,
            current_epoch_stats=CurrentEpochStats(
                inference_count="10",
                missed_requests="1",
                earned_coins="0",
                rewarded_coins="0",
                burned_coins="0",
                validated_inferences="10",
                invalidated_inferences="0"
            )
        )
    ]
    
    response = InferenceResponse(
        epoch_id=1,
        height=1000,
        participants=participants,
        is_current=True
    )
    
    assert response.epoch_id == 1
    assert response.height == 1000
    assert len(response.participants) == 1
    assert response.is_current is True


def test_inference_response_serialization():
    participants = [
        ParticipantStats(
            index="participant_1",
            address="gonka1abc...",
            weight=100,
            current_epoch_stats=CurrentEpochStats(
                inference_count="10",
                missed_requests="1",
                earned_coins="0",
                rewarded_coins="0",
                burned_coins="0",
                validated_inferences="10",
                invalidated_inferences="0"
            )
        )
    ]
    
    response = InferenceResponse(
        epoch_id=1,
        height=1000,
        participants=participants,
        cached_at="2025-10-19T12:00:00Z"
    )
    
    json_data = response.model_dump_json()
    assert "epoch_id" in json_data
    assert "participants" in json_data
    assert "missed_rate" in json_data


def test_model_from_real_data():
    test_data_dir = Path(__file__).parent.parent.parent / "test_data"
    files = list(test_data_dir.glob("all_participants_height_*.json"))
    
    if not files:
        pytest.skip("No test data available")
    
    with open(files[0]) as f:
        data = json.load(f)
    
    participants_data = data.get("participant", [])
    if not participants_data:
        pytest.skip("No participant data in file")
    
    first_participant = participants_data[0]
    
    participant = ParticipantStats(
        index=first_participant["index"],
        address=first_participant["address"],
        weight=first_participant["weight"],
        inference_url=first_participant.get("inference_url"),
        status=first_participant.get("status"),
        current_epoch_stats=CurrentEpochStats(**first_participant["current_epoch_stats"])
    )
    
    assert participant.index
    assert participant.address
    assert participant.missed_rate >= 0.0

