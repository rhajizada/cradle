# Examples

## Pull Image Alias

```yaml
version: 1
aliases:
  rocky:
    image:
      pull:
        ref: rockylinux:9
    run:
      username: ${USER:-user}
      uid: ${UID:-1000}
      gid: ${GID:-1000}
      cmd: ["/bin/bash"]
      network: host
      tty: true
      stdin_open: true
      auto_remove: true
```

## Build Image Alias

```yaml
version: 1
aliases:
  devbox:
    image:
      build:
        cwd: ./images/devbox
        dockerfile: Dockerfile
        args:
          USERNAME: ${USER}
          UID: ${UID:-1000}
        platforms: ["linux/amd64"]
    run:
      username: ${USER:-user}
      uid: ${UID:-1000}
      gid: ${GID:-1000}
      workdir: /home/${USER}
      env:
        TERM: xterm-256color
        COLORTERM: truecolor
      cmd: ["/bin/bash", "-l"]
      mounts:
        - type: bind
          source: ${HOME}
          target: /home/${USER}
```

## Detached Run

```yaml
version: 1
aliases:
  background:
    image:
      pull:
        ref: alpine:3.20
    run:
      username: ${USER:-user}
      uid: ${UID:-1000}
      gid: ${GID:-1000}
      cmd: ["/bin/sh", "-c", "sleep 3600"]
      attach: false
      auto_remove: true
```
