import sys
sys.path.append('../src')
sys.path.append('../../common/src')

import os
import copy
import json
from time import sleep
from concurrent.futures import ThreadPoolExecutor

import requests
from transformers import AutoTokenizer

from validation.runner import run_validation
from validation.prompts import preload_all_language_prompts, slice_mixed_language_prompts_with_langs
from validation.data import ModelInfo, RequestParams, ServerConfig, RunParams, InferenceValidationRun
from model_presets import QWEN3_30B_INT4, QWEN3_30B_FP8, QWEN3_235B_FP8, QWEN3_235B_INT4


N_PROMPTS = 1000
run_params_high_temp = RunParams(
    exp_name='235_repro',
    output_path='../data/inference_results',
    n_prompts=N_PROMPTS,
    timeout=1800,
    tokenizer_model_name='unsloth/llama-3-8b-Instruct',
    request=RequestParams(
        max_tokens=3000,
        temperature=0.99,
        seed=42,
        top_logprobs=4,
    ),
)

def get_run_params(temp, prompts):
    params = copy.deepcopy(run_params_high_temp)
    params.request.temperature = temp
    params.n_prompts = prompts
    return params

model_qwen3_fp8 = QWEN3_30B_FP8
model_qwen3_int4 = QWEN3_30B_INT4
model_qwen235b_fp8 = QWEN3_235B_FP8
model_qwen235b_int4 = QWEN3_235B_INT4

server_4h100_1 = ServerConfig(
    ip='45.135.56.10',
    inference_port='30961',
    node_port='23687',
    gpu='4xH100',
)

server_4h100_2 = ServerConfig(
    ip='64.62.194.210',
    inference_port='19704',
    node_port='31128',
    gpu='4xH100',
)

server_4h200_1 = ServerConfig(
    ip='208.64.254.72',
    inference_port='20336',
    node_port='20443',
    gpu='4xH200',
)

# server_3090_1 = ServerConfig(
#     ip='72.49.201.130',
#     inference_port='46726',
#     node_port='47177',
#     gpu='2x3090',
# )


# server_3090_2 = ServerConfig(
#     ip='24.124.32.70',
#     inference_port='49622',
#     node_port='49615',
#     gpu='2x3090',
# )


langs = ("en", "sp","ch", "hi", "ar")
runs = [
    # InferenceValidationRun(
    #     model_inference=model_qwen3_fp8,
    #     model_validation=model_qwen3_fp8,
    #     server_inference=server_h100_1,
    #     server_validation=server_h100_2,
    #     run_inference=get_run_params(0.99, N_PROMPTS),
    #     run_validation=get_run_params(0.99, N_PROMPTS),
    #     max_workers=10,
    # ),
    # InferenceValidationRun(
    #     model_inference=model_qwen3_int4,
    #     model_validation=model_qwen3_fp8,
    #     server_inference=server_h100_1,
    #     server_validation=server_h100_2,
    #     run_inference=get_run_params(0.7, N_PROMPTS//5),
    #     run_validation=get_run_params(0.7, N_PROMPTS//5),
    #     max_workers=10,
    # ),
    # InferenceValidationRun(
    #     model_inference=model_qwen235b_fp8,
    #     model_validation=model_qwen235b_fp8,
    #     server_inference=server_4h100_1,
    #     server_validation=server_4h200_1,
    #     run_inference=get_run_params(0.99, N_PROMPTS),
    #     run_validation=get_run_params(0.99, N_PROMPTS),
    #     max_workers=50,
    # ),
    # InferenceValidationRun(
    #     model_inference=model_qwen235b_int4,
    #     model_validation=model_qwen235b_fp8,
    #     server_inference=server_4h100_1,
    #     server_validation=server_4h100_2,
    #     run_inference=get_run_params(0.7, N_PROMPTS),
    #     run_validation=get_run_params(0.7, N_PROMPTS),
    #     max_workers=50,
    # ),
    InferenceValidationRun(
        model_inference=model_qwen235b_fp8,
        model_validation=model_qwen235b_fp8,
        server_inference=server_4h100_1,
        server_validation=server_4h200_1,
        run_inference=get_run_params(0.99, N_PROMPTS),
        run_validation=get_run_params(0.99, N_PROMPTS),
        max_workers=50,
    ),
]


def post_up(server: ServerConfig, payload, timeout: int):
    response = requests.post(server.up_url(), json=payload, timeout=timeout)
    response.raise_for_status()
    return server, response


def main():
    dataset = preload_all_language_prompts(langs=langs)
    for cfg in runs:
        tokenizer_model_name = cfg.run_inference.tokenizer_model_name
        _ = AutoTokenizer.from_pretrained(tokenizer_model_name)

        os.makedirs(cfg.run_inference.output_path, exist_ok=True)

        inference_payload = cfg.model_inference.to_deploy_payload()
        validation_payload = cfg.model_validation.to_deploy_payload()
        up_requests = [
            (cfg.server_inference, inference_payload),
            (cfg.server_validation, validation_payload),
        ]
        print(up_requests)
        with ThreadPoolExecutor(max_workers=len(up_requests)) as executor:
            futures = [
                executor.submit(post_up, srv, payload, cfg.run_inference.timeout)
                for srv, payload in up_requests
            ]
            for future in futures:
                _, response = future.result()
                print(response.status_code)
                print(response.text)

        sleep(5)

        inference_model_info = ModelInfo(
            url=cfg.server_inference.inference_url(),
            name=cfg.model_inference.model,
            deploy_params={
                "GPU": cfg.server_inference.gpu,
                "precision": cfg.model_inference.precision,
            },
        )

        validation_model_info = ModelInfo(
            url=cfg.server_validation.inference_url(),
            name=cfg.model_validation.model,
            deploy_params={
                "GPU": cfg.server_validation.gpu,
                "precision": cfg.model_validation.precision,
            },
        )

        request_params = cfg.run_inference.request
        setting_name = cfg.setting_filename()
        data_path = f"{cfg.run_inference.output_path}/{setting_name}"
        n_prompts = cfg.run_inference.n_prompts

        # Save run configuration next to results in a human-readable JSON
        config_filename = f"{setting_name.rsplit('.', 1)[0]}_config.json"
        config_path = f"{cfg.run_inference.output_path}/{config_filename}"
        with open(config_path, 'w', encoding='utf-8') as f:
            json.dump(cfg.model_dump(), f, indent=2, ensure_ascii=False)

        prompts, languages = slice_mixed_language_prompts_with_langs(dataset, per_language_n=n_prompts//len(langs), langs=langs)
        _ = run_validation(
            prompts,
            languages=languages,
            inference_model=inference_model_info,
            validation_model=validation_model_info,
            request_params=request_params,
            max_workers=cfg.max_workers,
            output_path=data_path,
        )
        print(f"Completed run. Results saved to: {data_path}")


if __name__ == "__main__":
    main()

# vllm serve chriswritescode/Qwen3-235B-A22B-Instruct-2507-INT4-W4A16 \
#   --tensor-parallel-size 4 \
#   --max-model-len 3000 \
#   --gpu-memory-utilization 0.95 \
#   --port 8000
#   --enforce-eager
#   --enable-expert-parallel