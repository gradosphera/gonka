import httpx
from typing import List, Dict, Any, Optional
import logging

logger = logging.getLogger(__name__)


class GonkaClient:
    def __init__(self, base_urls: List[str], timeout: float = 30.0):
        self.base_urls = base_urls
        self.timeout = timeout
        self.current_url_index = 0
        
    def _get_current_url(self) -> str:
        return self.base_urls[self.current_url_index]
    
    def _rotate_url(self) -> None:
        self.current_url_index = (self.current_url_index + 1) % len(self.base_urls)
        logger.info(f"Rotated to URL: {self._get_current_url()}")
    
    async def _make_request(
        self, 
        path: str, 
        params: Optional[Dict[str, Any]] = None, 
        headers: Optional[Dict[str, str]] = None
    ) -> Dict[str, Any]:
        attempts = len(self.base_urls)
        last_error = None
        
        for attempt in range(attempts):
            url = self._get_current_url().rstrip('/') + '/' + path.lstrip('/')
            
            try:
                async with httpx.AsyncClient(timeout=self.timeout) as client:
                    logger.debug(f"Request to {url} with params {params}, headers {headers}")
                    response = await client.get(url, params=params, headers=headers)
                    response.raise_for_status()
                    return response.json()
            except Exception as e:
                last_error = e
                logger.warning(f"Request failed to {url}: {e}")
                self._rotate_url()
        
        raise Exception(f"All URLs failed. Last error: {last_error}")
    
    async def get_current_epoch_participants(self) -> Dict[str, Any]:
        return await self._make_request("/v1/epochs/current/participants")
    
    async def get_epoch_participants(self, epoch_id: int) -> Dict[str, Any]:
        return await self._make_request(f"/v1/epochs/{epoch_id}/participants")
    
    async def get_all_participants(self, height: Optional[int] = None) -> Dict[str, Any]:
        params = {"pagination.limit": "10000"}
        headers = {}
        
        if height is not None:
            headers["X-Cosmos-Block-Height"] = str(height)
        
        return await self._make_request(
            "/chain-api/productscience/inference/inference/participant",
            params=params,
            headers=headers if headers else None
        )
    
    async def get_latest_height(self) -> int:
        data = await self._make_request("/chain-rpc/status")
        return int(data["result"]["sync_info"]["latest_block_height"])
    
    async def discover_urls(self) -> List[str]:
        try:
            participants_data = await self.get_current_epoch_participants()
            participants = participants_data.get("active_participants", {}).get("participants", [])
            
            discovered = []
            for p in participants:
                inference_url = p.get("inference_url", "").rstrip('/')
                if inference_url and inference_url not in self.base_urls:
                    discovered.append(inference_url)
            
            logger.info(f"Discovered {len(discovered)} additional URLs")
            return discovered
        except Exception as e:
            logger.error(f"Failed to discover URLs: {e}")
            return []

