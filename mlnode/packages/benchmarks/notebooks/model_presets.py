from validation.data import ModelPreset


QWEN25_7B_INT8 = ModelPreset(
    model='RedHatAI/Qwen2.5-7B-Instruct-quantized.w8a16',
    precision='int8',
    dtype='float16',
    additional_args=[
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'hermes',
    ],
)


DEEPSEEK_R1_0528_FP8 = ModelPreset(
    model='deepseek-ai/DeepSeek-R1-0528',
    precision='fp8',
    dtype='auto',
    additional_args=[
        '--quantization', 'fp8',
        '--gpu-memory-utilization', '0.95',
        '--tensor-parallel-size', '8',  
        '--pipeline-parallel-size', '1', 
        '--enable-expert-parallel', 
        '--max_model_len', '10000', 
        '--enable-auto-tool-choice', 
        '--tool-call-parser', 'deepseek_v3',
        ],
)

DEEPSEEK_R1_0528_INT4 = ModelPreset(
    model='RedHatAI/DeepSeek-R1-0528-quantized.w4a16',
    precision='int4',
    dtype='auto',
    additional_args=[
        '--gpu-memory-utilization', '0.95',
        '--tensor-parallel-size', '8',
        '--pipeline-parallel-size', '1' ,
        '--enable-expert-parallel',
        '--max_model_len', '10000',
        '--enable-auto-tool-choice', 
        '--tool-call-parser', 'deepseek_v3',
        ],
)


GEMMA_3_27B_FP8 = ModelPreset(
    model='pytorch/gemma-3-27b-it-FP8',
    precision='fp8',
    dtype='auto',
    additional_args=[
        '--gpu-memory-utilization', '0.95',
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'pythonic',
        '--max_model_len', '10000',
    ],
)


GEMMA_3_27B_INT4 = ModelPreset(
    model='RedHatAI/gemma-3-27b-it-quantized.w4a16',
    precision='int4',
    dtype='auto',
    additional_args=[
        '--gpu-memory-utilization', '0.95',
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'pythonic',
        '--max_model_len', '10000',
    ],
)

QWEN3_30B_FP8 = ModelPreset(
    model='Qwen/Qwen3-30B-A3B-Instruct-2507-FP8',
    precision='fp8',
    dtype='float16',
    additional_args=[
        '--gpu-memory-utilization', '0.95',
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'hermes',
        '--max_model_len', '10000',
    ],
)

QWEN3_30B_INT4 = ModelPreset(
    model='cpatonn/Qwen3-30B-A3B-Instruct-2507-AWQ-4bit',
    precision='int4',
    dtype='float16',
    additional_args=[
        '--gpu-memory-utilization', '0.95',
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'hermes',
        '--max_model_len', '10000',
    ],
)

QWEN3_235B_FP8 = ModelPreset(
    model='Qwen/Qwen3-235B-A22B-Instruct-2507-FP8',
    precision='fp8',
    dtype='float16',
    additional_args=[
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'hermes',
        '--max_model_len', '240000',
    ],
)

QWEN3_235B_INT4 = ModelPreset(
    model='chriswritescode/Qwen3-235B-A22B-Instruct-2507-INT4-W4A16',
    precision='int4',
    dtype='float16',
    additional_args=[
        '--enable-auto-tool-choice',
        '--tool-call-parser', 'hermes',
        '--max_model_len', '240000',
    ],
)
    