FROM ubuntu:24.04

ARG USERNAME
ARG UID

COPY packages.txt /tmp/setup/packages

RUN apt-get update -y && \
  apt-get install -y $(cat /tmp/setup/packages) && \
  apt-get clean

RUN set -eux; \
  apt-get update; \
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  sudo \
  ca-certificates \
  ; \
  rm -rf /var/lib/apt/lists/*; \
  \
  if getent passwd "${UID}" >/dev/null; then \
  olduser="$(getent passwd "${UID}" | cut -d: -f1)"; \
  userdel -r "${olduser}" || userdel "${olduser}"; \
  fi; \
  \
  if id -u "${USERNAME}" >/dev/null 2>&1; then \
  userdel -r "${USERNAME}" || userdel "${USERNAME}"; \
  fi; \
  \
  useradd -m -s /bin/bash -u "${UID}" "${USERNAME}"; \
  usermod -aG sudo "${USERNAME}"; \
  \
  echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}"; \
  chmod 0440 "/etc/sudoers.d/${USERNAME}"


USER ${USERNAME}
ENV USERNAME=${USERNAME}
WORKDIR /home/${USERNAME}
