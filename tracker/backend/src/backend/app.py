import asyncio
import logging
import os
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from backend.router import router, set_inference_service
from backend.client import GonkaClient
from backend.database import CacheDB
from backend.service import InferenceService

logger = logging.getLogger(__name__)

background_task = None
inference_service_instance = None


async def poll_current_epoch():
    while True:
        try:
            if inference_service_instance:
                await inference_service_instance.get_current_epoch_stats(reload=True)
                logger.info("Background polling: fetched current epoch stats")
        except Exception as e:
            logger.error(f"Background polling error: {e}")
        
        await asyncio.sleep(30)


@asynccontextmanager
async def lifespan(app: FastAPI):
    global background_task, inference_service_instance
    
    inference_urls = os.getenv("INFERENCE_URLS", "http://node2.gonka.ai:8000").split(",")
    inference_urls = [url.strip() for url in inference_urls]
    
    db_path = os.getenv("CACHE_DB_PATH", "cache.db")
    
    logger.info(f"Initializing with URLs: {inference_urls}")
    logger.info(f"Database path: {db_path}")
    
    cache_db = CacheDB(db_path)
    await cache_db.initialize()
    
    client = GonkaClient(base_urls=inference_urls)
    inference_service_instance = InferenceService(client=client, cache_db=cache_db)
    
    set_inference_service(inference_service_instance)
    
    background_task = asyncio.create_task(poll_current_epoch())
    logger.info("Background polling task started")
    
    yield
    
    if background_task:
        background_task.cancel()
        try:
            await background_task
        except asyncio.CancelledError:
            logger.info("Background polling task cancelled")


app = FastAPI(lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:3000"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(router)

