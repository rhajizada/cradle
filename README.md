# Cradle

Cradle is a lightweight CLI for jumping into a Docker image as if it were a temporary dev environment. Point it at an image (or alias) and it builds/launches it with a consistent, preconfigured setup, then opens a ready-to-use interactive shell so you can test tools, reproduce issues, or explore a distro without setting anything up locally.

## Features

- YAML config with named images for pull or build.
- Environment variable substitution in config.
- Interactive attach with TTY, resize, and signal handling.
- Mounts, ports, and resource limits.
- Clean, colorized pull/build output.

## Install

Build locally:

```sh
go build -o bin/cradle ./
```

Print version:

```sh
./bin/cradle -V
```

## Usage

```sh
cradle [--config <path>] <command>
```

Commands:

- `build <alias|all>` - pull or build images
- `run <alias>` - run alias interactively

## Config

Default config path:

- `${XDG_CONFIG_HOME}/cradle/config.yaml` (preferred)
- `$HOME/.config/cradle/config.yaml` (fallback)

Defaults in config:

- `image.build.dockerfile`: `Dockerfile`
- `run.tty`: `true`
- `run.stdin_open`: `true`
- `run.attach`: `true`
- `run.auto_remove`: `true`
- All other optional strings default to empty, booleans to `false`, lists/maps to empty.

Config supports environment variable substitution (see `docs/CONFIG.md`).

## Quick Start

Create a config at `${XDG_CONFIG_HOME}/cradle/config.yaml`:

```yaml
version: 1
aliases:
  ubuntu:
    image:
      pull:
        ref: ubuntu:24.04
    run:
      username: ${USER:-user}
      uid: ${UID:-1000}
      gid: ${GID:-1000}
      cmd: ["/bin/bash"]
      tty: true
      stdin_open: true
      auto_remove: true
      network: host
```

Run:

```sh
cradle run ubuntu
```

## Docs

- [Config reference](docs/CONFIG.md)
