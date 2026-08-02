# Godot Pacman fixture image

This image is the first license-free Pacman runtime environment. It uses a
Godot 4 Linux image, copies `pkg/godot-pacman` into the fixture project, and
starts Godot headless. The runtime has no display or GPU requirement.

Build locally or in CI:

```sh
docker build \
  --build-arg GODOT_IMAGE=barichello/godot:4.3 \
  -f deploy/godot-pacman-fixture/Containerfile \
  -t jangolova/godot-pacman-fixture:local .
```

For production or reproducible CI, mirror and pin the Godot image by digest in
your private registry. Pacman tokens should be injected at runtime, never put
in the image or project files.
