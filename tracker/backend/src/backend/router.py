from fastapi import APIRouter, HTTPException, Query
from typing import Optional, Any
from backend.models import InferenceResponse, ParticipantDetailsResponse, TimelineResponse

router = APIRouter(prefix="/v1")

inference_service: Optional[Any] = None


def set_inference_service(service):
    global inference_service
    inference_service = service


@router.get("/hello")
def hello():
    return {"message": "hello"}


@router.get("/inference/current", response_model=InferenceResponse)
async def get_current_inference_stats(reload: bool = False):
    if inference_service is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    
    try:
        return await inference_service.get_current_epoch_stats(reload=reload)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to fetch current epoch stats: {str(e)}")


@router.get("/inference/epochs/{epoch_id}", response_model=InferenceResponse)
async def get_epoch_inference_stats(epoch_id: int, height: Optional[int] = None):
    if inference_service is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    
    if epoch_id < 1:
        raise HTTPException(status_code=400, detail="Invalid epoch ID")
    
    if height is not None and height < 1:
        raise HTTPException(status_code=400, detail="Invalid height")
    
    try:
        return await inference_service.get_historical_epoch_stats(epoch_id, height=height)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to fetch epoch {epoch_id} stats: {str(e)}")


@router.get("/participants/{participant_id}", response_model=ParticipantDetailsResponse)
async def get_participant_details(
    participant_id: str,
    epoch_id: int = Query(..., description="Epoch ID (required)"),
    height: Optional[int] = Query(None, description="Block height (optional)")
):
    if inference_service is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    
    if epoch_id < 1:
        raise HTTPException(status_code=400, detail="Invalid epoch ID")
    
    if height is not None and height < 1:
        raise HTTPException(status_code=400, detail="Invalid height")
    
    try:
        details = await inference_service.get_participant_details(
            participant_id=participant_id,
            epoch_id=epoch_id,
            height=height
        )
        
        if details is None:
            raise HTTPException(
                status_code=404,
                detail=f"Participant {participant_id} not found in epoch {epoch_id}"
            )
        
        return details
        
    except HTTPException:
        raise
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Failed to fetch participant details: {str(e)}"
        )


@router.get("/timeline", response_model=TimelineResponse)
async def get_timeline():
    if inference_service is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    
    try:
        return await inference_service.get_timeline()
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to fetch timeline: {str(e)}")

