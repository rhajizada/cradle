# Config Reference

Cradle reads YAML config and expands environment variables before parsing. Unknown fields are errors.

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

- `ref` (string, required) - image reference, e.g. `ubuntu:24.04`.

#### image.build

- `cwd` (string, required) - build context directory.
- `dockerfile` (string, optional) - defaults to `Dockerfile`.
- `args` (map, optional) - build args.
- `target` (string, optional) - build target stage.
- `labels` (map, optional) - labels.
- `pull` (bool, optional) - pull parent image.
- `no_cache` (bool, optional) - disable build cache.
- `cache_from` (list, optional) - cache sources.
- `network` (string, optional) - build network mode.
- `extra_hosts` (list, optional) - extra host entries.
- `platforms` (list, optional) - build platforms (e.g. `linux/amd64`).

### run

Identity:

- `username` (string, required) - informational; used by your image/useradd.
- `uid` (int, required) - numeric uid.
- `gid` (int, required) - numeric gid.

I/O:

- `tty` (bool, optional) - default `true`.
- `stdin_open` (bool, optional) - default `true`.
- `attach` (bool, optional) - default `true`. If false, run detached.
- `auto_remove` (bool, optional) - default `true`.

Identity and hostname:

- `name` (string, optional) - container name.
- `hostname` (string, optional) - container hostname.

Process:

- `workdir` (string, optional) - working directory.
- `env` (map, optional) - environment variables.
- `entrypoint` (list, optional) - entrypoint override.
- `cmd` (list, optional) - command override.

Networking:

- `network` (string, optional) - `host|bridge|none|<net>`.
- `ports` (list, optional) - port mappings:
  - `"80"`
  - `"8080:80"`
  - `"127.0.0.1:2222:22"`
  - `"[::1]:2222:22"`
- `extra_hosts` (list, optional) - extra host entries.

Mounts:

- `mounts` (list, optional)
  - `type` (string, required) - `bind|volume|tmpfs`
  - `source` (string, required for bind/volume) - path or volume name
  - `target` (string, required) - container path
  - `readonly` (bool, optional)

Resources:

- `resources.cpus` (number, optional) - CPU count (maps to NanoCPUs).
- `resources.memory` (string, optional) - memory limit (e.g. `512m`, `2g`).
- `resources.shm_size` (string, optional) - shm size.

Other:

- `privileged` (bool, optional) - run privileged.
- `restart` (string, optional) - `no|on-failure|always|unless-stopped`.
- `platform` (string, optional) - runtime platform `os/arch[/variant]`.

## Example

```yaml
version: 1
aliases:
  ubuntu:
    image:
      build:
        cwd: ./images/ubuntu
        dockerfile: Dockerfile
        args:
          USERNAME: ${USER}
          UID: ${UID:-1000}
    run:
      username: ${USER:-user}
      uid: ${UID:-1000}
      gid: ${GID:-1000}
      workdir: /home/${USER}
      env:
        TERM: xterm-256color
        COLORTERM: truecolor
      cmd: ["/bin/bash", "-l"]
      network: host
      auto_remove: true
      attach: true
      mounts:
        - type: bind
          source: ${HOME}
          target: /home/${USER}
```
