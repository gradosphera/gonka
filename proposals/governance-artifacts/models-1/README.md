# New supported models

The proposal introduces 3 new models:

- `Qwen/Qwen3-235B-A22B-Instruct-2507-FP8` - first big model on Gonka chain.
- `Qwen/Qwen3-32B-FP8` - fresh medium size model, replacement for `Qwen/QwQ-32B`
- `RedHatAI/Qwen2.5-7B-Instruct-quantized.w8a16` - replacement for `Qwen/Qwen2.5-7B-Instruct` (minor improvement to get rid of fully dynamic quantization)

## New values estimations

The methodology for computing the thresholds is detailed in the [thresholds_sep2025.ipynb](./thresholds_sep2025.ipynb). A reproducible version of this notebook is available in the project repository at [/mlnode/packages/benchmarks/notebooks/thresholds_sep2025.ipynb](../../..//mlnode/packages/benchmarks/notebooks/thresholds_sep2025.ipynb).

The data used for our experiments is available for verification [link](https://drive.google.com/drive/folders/1ehpcVC0pGw0XwrchXZUxTTRy1KdhBxrz?usp=drive_link). For maximum confidence, participants are encouraged to recompute the thresholds independently using the provided notebook.


- All experiments were conducted using MLNode `v3.0.9`, which is the current version in the main branch..
- `Qwen/Qwen3-235B-A22B-Instruct-2507-FP8` is proposed with `--max-model-len 240000`.  This value is optimized for deployment on systems with 320GB of VRAM, allowing, for example, two instances of the model to run on a standard 8xH100 server.


## Release process

If this proposal is approved, node operators will be able to modify their MLNodes config to switch to new models. Transition can be done asyncronously. 

Detailed instructions for a seamless transition will be published on the official project website and announced through all relevant community channels.
