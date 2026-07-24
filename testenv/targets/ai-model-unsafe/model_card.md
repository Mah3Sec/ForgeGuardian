# FORGEGUARDIAN TEST FIXTURE — Unsafe AI Model

**Model name:** forgeguardian-test-unsafe-model  
**Format:** pickle (.pkl)  
**License:** none

## Description

This model fixture tests ForgeGuardian's AI model scanner:
- Serialized using Python `pickle` (unsafe format — allows arbitrary code execution on load)
- No `safetensors` equivalent provided
- Model card missing: bias evaluation, training data provenance, safety evaluation
- No content filtering policy documented

## Files

- `model.pkl` — pickle-serialized weights (unsafe)
- `config.json` — model config

## Warning

Pickle files from untrusted sources can execute arbitrary code when loaded with `torch.load()` or `pickle.load()`. ForgeGuardian's AI model scanner should flag this as HIGH severity.
