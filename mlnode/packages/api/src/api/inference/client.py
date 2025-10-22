import requests
from common.wait import wait_for_server


class InferenceClient:
    def __init__(self, base_url):
        self.base_url = base_url

    def _request(self, method, endpoint, json=None):
        url = f"{self.base_url}/api/v1{endpoint}"
        response = getattr(requests, method)(url, json=json)
        try:
            response.raise_for_status()
        except requests.HTTPError as e:
            print(f"HTTP Error: {e}")
            print(f"Response content: {response.text}")
            raise
        return response.json()

    def wait_for_server(self, timeout=30):
        wait_for_server(f"{self.base_url}", timeout)
    
    def inference_setup(self, model, dtype, additional_args=[]):
        """Setup inference with automatic restart if already running.
        
        If VLLM is already running or starting, this will automatically stop it first
        and then start with the new configuration.
        """
        self.wait_for_server()
        
        url = f"{self.base_url}/api/v1/inference/up"
        response = requests.post(url, json={
            "model": model,
            "dtype": dtype,
            "additional_args": additional_args,
        })
        
        # If we get a 409 conflict (already running or starting), stop first and retry
        if response.status_code == 409:
            print(f"VLLM already running/starting, stopping first...")
            self.inference_down()
            # Retry the setup
            response = requests.post(url, json={
                "model": model,
                "dtype": dtype,
                "additional_args": additional_args,
            })
        
        try:
            response.raise_for_status()
        except requests.HTTPError as e:
            print(f"HTTP Error: {e}")
            print(f"Response content: {response.text}")
            raise
        
        return response.json()

    def inference_down(self):
        return self._request("post", "/inference/down")
