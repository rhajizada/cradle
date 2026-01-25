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
  user: ${USER:-user}
  uid: ${UID:-1000}
  gid: ${GID:-1000}
```

## Path Resolution

Paths are resolved relative to the config file directory:

- `image.build.cwd`
- `run.volumes[].source` when `type: bind`

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
- `policy` (string, optional) - `always|if_missing|never` (default `always`).

#### image.build

- `cwd` (string, required unless `remote_context` is set) - build context directory.
  Example: `cwd: ./images/devbox`
- `policy` (string, optional) - `always|if_missing|never` (default `always`).
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
- `tags` (list, optional) - extra image tags (in addition to the alias tag).
- `suppress_output` (bool, optional) - suppress build output.
- `remote_context` (string, optional) - build context URL (e.g. git). If set, `cwd` can be empty.
- `remove` (bool, optional) - remove intermediate containers (default `true`).
- `force_remove` (bool, optional) - always remove intermediate containers (default `true`).
- `isolation` (string, optional) - container isolation (platform-specific).
- `cpuset_cpus` (string, optional) - CPUs in which to allow execution.
- `cpuset_mems` (string, optional) - memory nodes in which to allow execution.
- `cpu_shares` (int, optional) - CPU share weighting.
- `cpu_quota` (int, optional) - CPU quota in microseconds.
- `cpu_period` (int, optional) - CPU period in microseconds.
- `memory` (int, optional) - memory limit in bytes.
- `memory_swap` (int, optional) - total memory limit (memory + swap) in bytes.
- `cgroup_parent` (string, optional) - cgroup parent.
- `shm_size` (int, optional) - shared memory size in bytes.
- `ulimits` (list, optional) - ulimits.
  Example:
  ```yaml
  ulimits:
    - name: nofile
      soft: 1024
      hard: 2048
  ```
- `auth_configs` (map, optional) - registry auth configs keyed by host.
  Example:
  ```yaml
  auth_configs:
    ghcr.io:
      username: ${GITHUB_USER}
      password: ${GITHUB_TOKEN}
  ```
- `squash` (bool, optional) - squash build layers.
- `security_opt` (list, optional) - security options.
- `build_id` (string, optional) - build identifier for cancellation.
- `outputs` (list, optional) - BuildKit outputs.
  Example:
  ```yaml
  outputs:
    - type: local
      attrs:
        dest: ./out
  ```

### run

Identity:

- `user` (string, optional) - user or user:group string.
  Example: `user: ${USER}`
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

- `work_dir` (string, optional) - working directory.
  Example: `work_dir: /workspace`
- `domain_name` (string, optional) - domain name.
- `user` (string, optional) - user or user:group string.
- `group_add` (list, optional) - supplementary groups.
- `labels` (map, optional) - container labels.
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

- `network_mode` (string, optional) - `host|bridge|none|<net>`.
  Example: `network_mode: host`
- `networks` (map, optional) - attach to named networks with optional aliases.
  Example:
  ```yaml
  networks:
    default:
      aliases: ["app"]
  ```
- `ports` (list, optional) - port mappings:
  - `"80"`
  - `"8080:80"`
  - `"127.0.0.1:2222:22"`
  - `"[::1]:2222:22"`
- `expose` (list, optional) - expose container ports without host bindings.
- `extra_hosts` (list, optional) - extra host entries.
  Example:
  ```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
  ```
- `dns` (list, optional) - DNS servers.
- `dns_search` (list, optional) - DNS search domains.
- `dns_opt` (list, optional) - DNS options.
- `ipc` (string, optional) - IPC namespace.
- `pid` (string, optional) - PID namespace.
- `uts` (string, optional) - UTS namespace.
- `runtime` (string, optional) - container runtime.

Mounts:

- `volumes` (list, optional)
  - `type` (string, required) - `bind|volume|tmpfs`
  - `source` (string, required for bind/volume) - path or volume name
  - `target` (string, required) - container path
  - `read_only` (bool, optional)
  Example:
  ```yaml
  volumes:
    - type: bind
      source: ./src
      target: /workspace
      read_only: false
    - type: volume
      source: npm-cache
      target: /home/node/.npm
  ```

Resources:

- `resources.cpus` (number, optional) - CPU count (maps to NanoCPUs).
  Example: `cpus: 2`
- `resources.cpu_shares` (int, optional) - CPU shares.
- `resources.cpu_quota` (int, optional) - CPU quota.
- `resources.cpu_period` (int, optional) - CPU period.
- `resources.cpuset_cpus` (string, optional) - CPU set.
- `resources.cpuset_mems` (string, optional) - memory node set.
- `resources.memory` (string, optional) - memory limit (e.g. `512m`, `2g`).
  Example: `memory: 2g`
- `resources.memory_reservation` (string, optional) - soft memory limit.
- `resources.memory_swap` (string, optional) - memory+swap limit.
- `resources.pids_limit` (int, optional) - PIDs limit.
- `resources.oom_kill_disable` (bool, optional) - disable OOM killer.
- `resources.cgroup_parent` (string, optional) - cgroup parent.
- `resources.shm_size` (string, optional) - shm size.
  Example: `shm_size: 1g`

Other:

- `privileged` (bool, optional) - run privileged.
  Example: `privileged: true`
- `read_only` (bool, optional) - read-only root filesystem.
- `cap_add` (list, optional) - add Linux capabilities.
- `cap_drop` (list, optional) - drop Linux capabilities.
- `security_opt` (list, optional) - security options.
- `sysctls` (map, optional) - sysctls.
- `ulimits` (list, optional) - ulimits.
- `tmpfs` (list, optional) - tmpfs mounts (e.g. `/run:rw,noexec`).
- `devices` (list, optional) - device mappings (e.g. `/dev/ttyUSB0:/dev/ttyUSB0:rwm`).
- `stop_signal` (string, optional) - stop signal.
- `stop_grace_period` (string, optional) - duration before force stop (e.g. `30s`).
- `healthcheck` (object, optional) - healthcheck configuration.
- `logging` (object, optional) - log driver config (driver/options).
- `restart` (string, optional) - `no|on-failure|always|unless-stopped`.
  Example: `restart: unless-stopped`
- `platform` (string, optional) - runtime platform `os/arch[/variant]`.
  Example: `platform: linux/amd64`

## Notes

- Relative paths in `image.build.cwd` and `run.volumes[].source` are resolved from the config file directory.
- Set `run.auto_remove: false` to keep containers for reuse across runs.
- If you override `run.name`, Cradle uses it to identify the container.
