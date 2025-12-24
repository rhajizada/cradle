# Commands

## cradle

```sh
cradle [--config <path>] <command>
```

Global flags:

- `-c, --config` - config file (default XDG path)
- `-V, --version` - print version

## aliases / ls

List aliases and their image sources.

```sh
cradle aliases
cradle ls
```

## build <alias|all>

Build or pull images. If an alias uses `image.pull`, it only pulls.

```sh
cradle build ubuntu
cradle build all
```

## run <alias>

Ensure the image exists, create the container, and attach interactively unless `run.attach` is false.

```sh
cradle run ubuntu
```
