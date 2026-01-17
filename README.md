# cradle

[![CI](https://github.com/rhajizada/cradle/actions/workflows/ci.yml/badge.svg)](https://github.com/rhajizada/cradle/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.25-blue.svg)
![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=fff)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![coverage](https://signum.rhajizada.dev/api/badges/41b351aa-7fbd-4f4e-a3ae-f46875c940fc)

**cradle** is a lightweight CLI for launching and attaching to predefined Docker containers. Define images and runtime settings in a YAML file, then build/pull, start, and attach with consistent mounts, env, and resources.

## Why **cradle**?

- Single config for repeatable dev shells.
- Pull or build images per alias.
- Interactive attach with proper TTY handling.
- Clean container reuse when `auto_remove` is false.

## Quick Start

Build:

```sh
go build -o bin/cradle ./cmd/cradle
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
      tty: true
      stdin_open: true
      attach: true
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

## Docs & References

- [Configuration reference](docs/CONFIG.md)
- [Example configuration](examples/config.yaml)
