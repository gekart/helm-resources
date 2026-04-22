FROM node:20

ARG TZ
ENV TZ="$TZ"

ARG CLAUDE_CODE_VERSION=latest

# Install basic development tools and iptables/ipset
RUN apt-get update && apt-get install -y --no-install-recommends \
  less \
  git \
  procps \
  sudo \
  fzf \
  zsh \
  man-db \
  unzip \
  gnupg2 \
  gh \
  iptables \
  ipset \
  iproute2 \
  dnsutils \
  aggregate \
  jq \
  nano \
  vim \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Ensure default node user has access to /usr/local/share
RUN mkdir -p /usr/local/share/npm-global && \
  chown -R node:node /usr/local/share

ARG USERNAME=node

# Persist bash history.
RUN SNIPPET="export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  && mkdir /commandhistory \
  && touch /commandhistory/.bash_history \
  && chown -R $USERNAME /commandhistory

# Set `DEVCONTAINER` environment variable to help with orientation
ENV DEVCONTAINER=true

# Create workspace and config directories and set permissions
RUN mkdir -p /workspace /home/node/.claude && \
  chown -R node:node /workspace /home/node/.claude

WORKDIR /workspace

ARG GIT_DELTA_VERSION=0.18.2
RUN ARCH=$(dpkg --print-architecture) && \
  wget "https://github.com/dandavison/delta/releases/download/${GIT_DELTA_VERSION}/git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" && \
  sudo dpkg -i "git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" && \
  rm "git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb"

# Install Go toolchain into /usr/local/share/go (writable by the node user)
ARG GO_VERSION=1.26.2
RUN ARCH=$(dpkg --print-architecture) && \
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -O /tmp/go.tgz && \
  rm -rf /usr/local/share/go && \
  tar -C /usr/local/share -xzf /tmp/go.tgz && \
  rm /tmp/go.tgz
ENV GOPATH=/home/node/go
ENV PATH=/usr/local/share/go/bin:/home/node/go/bin:$PATH

# Install Helm 4 CLI
ARG HELM_VERSION=v4.1.4
RUN ARCH=$(dpkg --print-architecture) && \
  wget -q "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" -O /tmp/helm.tgz && \
  tar -C /tmp -xzf /tmp/helm.tgz && \
  install -m 0755 "/tmp/linux-${ARCH}/helm" /usr/local/bin/helm && \
  rm -rf /tmp/helm.tgz "/tmp/linux-${ARCH}"

# Set up non-root user
USER node
RUN mkdir -p /home/node/go/bin

# Install global packages
ENV NPM_CONFIG_PREFIX=/usr/local/share/npm-global
ENV PATH=$PATH:/usr/local/share/npm-global/bin

# Set the default shell to zsh rather than sh
ENV SHELL=/bin/zsh

# Set the default editor and visual
ENV EDITOR=nano
ENV VISUAL=nano

# Default powerline10k theme
ARG ZSH_IN_DOCKER_VERSION=1.2.0
RUN sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v${ZSH_IN_DOCKER_VERSION}/zsh-in-docker.sh)" -- \
  -p git \
  -p fzf \
  -a "source /usr/share/doc/fzf/examples/key-bindings.zsh" \
  -a "source /usr/share/doc/fzf/examples/completion.zsh" \
  -a "export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  -x

# Install Claude
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}


# Copy and set up firewall script
COPY init-firewall.sh /usr/local/bin/
USER root
RUN chmod +x /usr/local/bin/init-firewall.sh && \
  echo "node ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh" > /etc/sudoers.d/node-firewall && \
  chmod 0440 /etc/sudoers.d/node-firewall
RUN npx -y playwright install-deps
USER node
RUN npx -y playwright install chromium
RUN PLAYWRIGHT_CHROMIUM=$(find ~/.cache/ms-playwright -type f -path '*chromium-*/chrome-linux/*' -name 'chrome' | head -n1); claude mcp add playwright npx '@playwright/mcp@latest' --   --executable-path "$PLAYWRIGHT_CHROMIUM" --headless