# Cradle

Cradle is a lightweight CLI for jumping into preconfigured Docker environments. Define images and runtime settings in a YAML file, then build/pull, start, and attach with consistent mounts, env, and resources.

## Why Cradle

- Single config for repeatable dev shells.
- Pull or build images per alias.
- Interactive attach with proper TTY handling.
- Clean container reuse when `auto_remove` is false.

## Quick Start

Build:

```sh
go build -o bin/cradle ./
```

Install example configuration:

```sh
make config
```

Or create `${XDG_CONFIG_HOME}/cradle/config.yaml` manually:

```yaml
version: 1
aliases:
  ubuntu:
    image:
      pull:
        ref: ubuntu:24.04
    run:
      cmd: ["/bin/bash"]
      auto_remove: false
```

Run:

```sh
./bin/cradle run ubuntu
```

## Commands

| Command        | Description                              |
| -------------- | ---------------------------------------- |
| `build`        | Pull or build images                     |
| `ls`           | List aliases with image/container status |
| `run <alias>`  | Run alias                                |
| `stop <alias>` | Stop alias container                     |

## Configuration

Default config path:

- `${XDG_CONFIG_HOME}/cradle/config.yaml` (preferred)
- `$HOME/.config/cradle/config.yaml` (fallback)

Essentials:

- `aliases.<name>.image.pull.ref`: image reference to pull (e.g. `ubuntu:24.04`).
- `aliases.<name>.image.build`: build context and options (`cwd`, `dockerfile`, `args`, `target`).
- `aliases.<name>.run.cmd`: command to run in the container.
- `aliases.<name>.run.env`: map of environment variables.
- `aliases.<name>.run.mounts`: bind/volume/tmpfs mounts into the container.
- `aliases.<name>.run.ports`: port mappings (`host:container`).
- `aliases.<name>.run.auto_remove`: keep container after exit when `false`.

Defaults:

- `image.build.dockerfile`: `Dockerfile`
- `run.tty`: `true`
- `run.stdin_open`: `true`
- `run.attach`: `true`
- `run.auto_remove`: `false`


## Docs & References

- [Configuration reference](docs/CONFIG.md)
- [Example configuration](examples/config.yaml)
