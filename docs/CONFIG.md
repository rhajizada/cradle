# Configuration reference

Cradle reads YAML config and expands environment variables before parsing. Unknown fields are errors.

This file is the source of truth for what Cradle builds, pulls, and runs. Each alias defines:

- an image source (`pull` or `build`)
- runtime settings (`run`)

## File Location

Default:

- `${XDG_CONFIG_HOME}/cradle/config.yaml`
- `$HOME/.config/cradle/config.yaml` (fallback)

Override with `-c/--config`.

## Environment Variable Substitution

Cradle expands variables before YAML parsing. Supported forms:

- `${VAR}`
- `$VAR`
- `${VAR:-default}` (default if unset or empty)
- `${VAR-default}` (default if unset only)
- `\$` to escape `$`
- `$$` becomes literal `$`

Example:

```yaml
run:
  username: ${USER:-user}
  uid: ${UID:-1000}
  gid: ${GID:-1000}
```

## Path Resolution

Paths are resolved relative to the config file directory:

- `image.build.cwd`
- `run.mounts[].source` when `type: bind`

## Schema Overview

Top-level:

- `version` (int) - config version (currently `1`).
- `aliases` (map) - alias name to config.

### aliases.<name>

- `image` - image source (pull or build).
- `run` - runtime settings.

### image

Exactly one of `pull` or `build` is required.

#### image.pull

- `ref` (string, required) - image reference.
  Example: `ref: ubuntu:24.04`

#### image.build

- `cwd` (string, required) - build context directory.
  Example: `cwd: ./images/devbox`
- `dockerfile` (string, optional) - defaults to `Dockerfile`.
  Example: `dockerfile: Dockerfile.dev`
- `args` (map, optional) - build args.
  Example:
  ```yaml
  args:
    UID: ${UID}
    GID: ${GID}
  ```
- `target` (string, optional) - build target stage.
  Example: `target: runtime`
- `labels` (map, optional) - labels.
  Example:
  ```yaml
  labels:
    org.example.role: dev
  ```
- `pull` (bool, optional) - pull parent image.
  Example: `pull: true`
- `no_cache` (bool, optional) - disable build cache.
  Example: `no_cache: true`
- `cache_from` (list, optional) - cache sources.
  Example:
  ```yaml
  cache_from:
    - ghcr.io/org/app:cache
  ```
- `network` (string, optional) - build network mode.
  Example: `network: host`
- `extra_hosts` (list, optional) - extra host entries.
  Example:
  ```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
  ```
- `platforms` (list, optional) - build platforms (e.g. `linux/amd64`).
  Example:
  ```yaml
  platforms:
    - linux/amd64
    - linux/arm64
  ```

### run

Identity:

- `username` (string, optional) - informational; used by your image/useradd.
  Example: `username: ${USER}`
- `uid` (int, optional) - numeric uid; if set, `gid` must also be set.
  Example: `uid: ${UID}`
- `gid` (int, optional) - numeric gid; if set, `uid` must also be set.
  Example: `gid: ${GID}`
- If `uid`/`gid` are omitted, the container uses the image's default `USER`.

I/O:

- `tty` (bool, optional) - default `false`.
  Example: `tty: true`
- `stdin_open` (bool, optional) - default `false`.
  Example: `stdin_open: true`
- `attach` (bool, optional) - default `false`. If false, run detached.
  Example: `attach: false`
- `auto_remove` (bool, optional) - default `false`.
  Example: `auto_remove: false`

Identity and hostname:

- `name` (string, optional) - container name.
  Example: `name: my-devbox`
- `hostname` (string, optional) - container hostname.
  Example: `hostname: devbox`

Process:

- `workdir` (string, optional) - working directory.
  Example: `workdir: /workspace`
- `env` (map, optional) - environment variables.
  Example:
  ```yaml
  env:
    FOO: bar
    NODE_ENV: development
  ```
- `entrypoint` (list, optional) - entrypoint override.
  Example: `entrypoint: ["/bin/bash", "-lc"]`
- `cmd` (list, optional) - command override.
  Example: `cmd: ["bash"]`

Networking:

- `network` (string, optional) - `host|bridge|none|<net>`.
  Example: `network: host`
- `ports` (list, optional) - port mappings:
  - `"80"`
  - `"8080:80"`
  - `"127.0.0.1:2222:22"`
  - `"[::1]:2222:22"`
- `extra_hosts` (list, optional) - extra host entries.
  Example:
  ```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
  ```

Mounts:

- `mounts` (list, optional)
  - `type` (string, required) - `bind|volume|tmpfs`
  - `source` (string, required for bind/volume) - path or volume name
  - `target` (string, required) - container path
  - `readonly` (bool, optional)
  Example:
  ```yaml
  mounts:
    - type: bind
      source: ./src
      target: /workspace
      readonly: false
    - type: volume
      source: npm-cache
      target: /home/node/.npm
  ```

Resources:

- `resources.cpus` (number, optional) - CPU count (maps to NanoCPUs).
  Example: `cpus: 2`
- `resources.memory` (string, optional) - memory limit (e.g. `512m`, `2g`).
  Example: `memory: 2g`
- `resources.shm_size` (string, optional) - shm size.
  Example: `shm_size: 1g`

Other:

- `privileged` (bool, optional) - run privileged.
  Example: `privileged: true`
- `restart` (string, optional) - `no|on-failure|always|unless-stopped`.
  Example: `restart: unless-stopped`
- `platform` (string, optional) - runtime platform `os/arch[/variant]`.
  Example: `platform: linux/amd64`

## Notes

- Relative paths in `image.build.cwd` and `run.mounts[].source` are resolved from the config file directory.
- Set `run.auto_remove: false` to keep containers for reuse across runs.
- If you override `run.name`, Cradle uses it to identify the container.
