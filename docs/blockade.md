# Blockade

Blockade is Jangolova's observation subsystem. It runs pixel-oriented models
locally or through caller-owned model services and returns normalized visual
observations. It does not perform interaction or presentation.

```text
pixels → Blockade engine → observations → Grimlock → approved action
                                      ↘ Cymonkey / Pacman
```

## Model registration

Grimlock is the model registry and orchestration boundary, but models have
roles:

- `reasoning`: text/agent models used to plan and select tools;
- `vision`: YOLO, SAM, OCR, and similar forward-pass models;
- `multimodal`: VLMs that accept pixels and language and may also reason.

A vision model does not need to understand Grimlock or Blockade protocols.
Grimlock or a Blockade provider adapter translates the normalized
`blockade.observation/v1alpha1` request into the model's native API. This keeps
YOLO, SAM, ONNX, OpenVINO, TensorRT, and VLM implementations replaceable.

The intended flow is:

```text
Grimlock model registry
  ├─ reasoning model → agent loop
  ├─ vision model    → Blockade adapter → detections/masks
  └─ multimodal      → Blockade adapter → grounded observations/reasoning
```

The first implementation uses an external Ultralytics YOLO/SAM worker. Future
Blockade backends can use ONNX Runtime, OpenVINO, TensorRT, or a VLM gateway
without changing Grimlock's observation tool.
