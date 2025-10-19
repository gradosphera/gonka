import httpx
import base64
import hashlib
from typing import List, Dict, Any, Optional
import logging
import time

logger = logging.getLogger(__name__)

BECH32_CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"


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
    
    async def get_all_validators(self, height: Optional[int] = None) -> List[Dict[str, Any]]:
        validators = []
        next_key = ""
        
        while True:
            params = {"pagination.limit": "200"}
            if next_key:
                params["pagination.key"] = next_key
            
            headers = {}
            if height is not None:
                headers["X-Cosmos-Block-Height"] = str(height)
            
            data = await self._make_request(
                "/chain-api/cosmos/staking/v1beta1/validators",
                params=params,
                headers=headers if headers else None
            )
            
            validators.extend(data.get("validators", []))
            next_key = data.get("pagination", {}).get("next_key") or ""
            
            if not next_key:
                break
        
        logger.info(f"Fetched {len(validators)} validators")
        return validators
    
    async def get_signing_info(self, valcons_addr: str, height: Optional[int] = None) -> Optional[Dict[str, Any]]:
        try:
            headers = {}
            if height is not None:
                headers["X-Cosmos-Block-Height"] = str(height)
            
            data = await self._make_request(
                f"/chain-api/cosmos/slashing/v1beta1/signing_infos/{valcons_addr}",
                headers=headers if headers else None
            )
            return data.get("val_signing_info")
        except Exception as e:
            logger.warning(f"Failed to get signing info for {valcons_addr}: {e}")
            return None
    
    @staticmethod
    def pubkey_to_valcons(pubkey_b64: str, hrp: str = "gonkavalcons") -> str:
        def _polymod(values: List[int]) -> int:
            generators = [0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3]
            checksum = 1
            for value in values:
                top = checksum >> 25
                checksum = ((checksum & 0x1FFFFFF) << 5) ^ value
                for i in range(5):
                    if (top >> i) & 1:
                        checksum ^= generators[i]
            return checksum
        
        def _hrp_expand(hrp: str) -> List[int]:
            return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]
        
        def _create_checksum(hrp: str, data: List[int]) -> List[int]:
            polymod = _polymod(_hrp_expand(hrp) + data + [0, 0, 0, 0, 0, 0]) ^ 1
            return [(polymod >> 5 * (5 - i)) & 31 for i in range(6)]
        
        def _bech32_encode(hrp: str, data: List[int]) -> str:
            return hrp + "1" + "".join(BECH32_CHARSET[d] for d in data + _create_checksum(hrp, data))
        
        def _convertbits(data: bytes, frombits: int, tobits: int, pad: bool = True) -> List[int]:
            accumulator = 0
            bits = 0
            result: List[int] = []
            max_value = (1 << tobits) - 1
            for byte in data:
                accumulator = (accumulator << frombits) | byte
                bits += frombits
                while bits >= tobits:
                    bits -= tobits
                    result.append((accumulator >> bits) & max_value)
            if pad and bits:
                result.append((accumulator << (tobits - bits)) & max_value)
            return result
        
        public_key = base64.b64decode(pubkey_b64)
        hex20 = hashlib.sha256(public_key).digest()[:20]
        data5 = _convertbits(hex20, 8, 5, pad=True)
        return _bech32_encode(hrp, data5)
    
    async def check_node_health(self, inference_url: str) -> Dict[str, Any]:
        if not inference_url:
            return {
                "is_healthy": False,
                "error_message": "No inference URL",
                "response_time_ms": None
            }
        
        health_url = inference_url.rstrip('/') + '/health'
        start_time = time.time()
        
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.get(health_url)
                response_time_ms = int((time.time() - start_time) * 1000)
                
                if response.status_code == 200:
                    return {
                        "is_healthy": True,
                        "error_message": None,
                        "response_time_ms": response_time_ms
                    }
                else:
                    return {
                        "is_healthy": False,
                        "error_message": f"HTTP {response.status_code}",
                        "response_time_ms": response_time_ms
                    }
        except Exception as e:
            return {
                "is_healthy": False,
                "error_message": str(e),
                "response_time_ms": None
            }

