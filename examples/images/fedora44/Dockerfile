FROM fedora:44

ARG USERNAME
ARG UID

COPY packages.txt /tmp/setup/packages

RUN dnf -y update && \
  dnf -y install $(cat /tmp/setup/packages) && \
  dnf clean all

RUN set -eux; \
  dnf -y install \
  sudo \
  ca-certificates \
  ; \
  dnf clean all; \
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
  usermod -aG wheel "${USERNAME}"; \
  \
  echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}"; \
  chmod 0440 "/etc/sudoers.d/${USERNAME}"


USER ${USERNAME}
ENV USERNAME=${USERNAME}
WORKDIR /home/${USERNAME}
