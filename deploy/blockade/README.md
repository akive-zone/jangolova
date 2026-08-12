# Blockade YOLO/SAM worker

This is the first local Blockade provider. It observes encoded images and
returns normalized detections and segmentation masks. Blockade owns pixel
observation; Jangolova and Grimlock remain responsible for interaction and
agent orchestration.

Build and run it from the repository root:

```sh
docker build -f deploy/blockade/Containerfile -t jangolova/blockade:yolo-sam .
docker run --rm -p 127.0.0.1:8091:8091 jangolova/blockade:yolo-sam
```

The first startup downloads the configured model weights unless they are
provided through a mounted cache. Override `BLOCKADE_YOLO_MODEL` and
`BLOCKADE_SAM_MODEL` when selecting different compatible Ultralytics weights.

`POST /v1/observe` accepts JSON with a base64-encoded `image` and returns the
`blockade.observation/v1alpha1` response. The Go client is in
`internal/blockade`. Set `JANGOLOVA_BLOCKADE_ENDPOINT=http://blockade:8091`
when starting Grimlock to advertise the read-only `blockade_observe` tool in
new sessions.
