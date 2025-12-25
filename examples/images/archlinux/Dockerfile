FROM archlinux:base-devel

ARG USERNAME
ARG UID

COPY packages.txt /tmp/setup/packages

RUN pacman -Syu --noconfirm && \
  pacman -S --noconfirm --needed $(cat /tmp/setup/packages) && \
  pacman -S --noconfirm --needed sudo ca-certificates && \
  pacman -Scc --noconfirm

RUN set -eux; \
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
  mkdir -p /etc/sudoers.d; \
  echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}"; \
  chmod 0440 "/etc/sudoers.d/${USERNAME}"


USER ${USERNAME}
ENV USERNAME=${USERNAME}
WORKDIR /home/${USERNAME}
