FROM opensuse/leap:16.0

ARG USERNAME
ARG UID

COPY packages.txt /tmp/setup/packages

RUN set -eux; \
  if ! zypper -n lr >/dev/null 2>&1 || [ "$(zypper -n lr | awk 'NR>2{print}' | wc -l)" -eq 0 ]; then \
  zypper -n ar -f https://download.opensuse.org/distribution/leap/16.0/repo/oss/ repo-oss; \
  zypper -n ar -f https://download.opensuse.org/distribution/leap/16.0/repo/non-oss/ repo-non-oss; \
  fi; \
  zypper -n --gpg-auto-import-keys refresh; \
  zypper -n install -y $(cat /tmp/setup/packages) sudo ca-certificates; \
  zypper -n clean -a

RUN set -eux; \
  if getent passwd "${UID}" >/dev/null; then \
  olduser="$(getent passwd "${UID}" | cut -d: -f1)"; \
  userdel -r "${olduser}" || userdel "${olduser}" || true; \
  fi; \
  if id -u "${USERNAME}" >/dev/null 2>&1; then \
  userdel -r "${USERNAME}" || userdel "${USERNAME}" || true; \
  fi; \
  useradd -m -s /bin/bash -u "${UID}" "${USERNAME}"; \
  \
  getent group wheel >/dev/null 2>&1 || groupadd -r wheel; \
  usermod -aG wheel "${USERNAME}"; \
  \
  mkdir -p /etc/sudoers.d; \
  echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}"; \
  chmod 0440 "/etc/sudoers.d/${USERNAME}"; \
  rm -rf /tmp/setup

USER ${USERNAME}
ENV USERNAME=${USERNAME}
WORKDIR /home/${USERNAME}
