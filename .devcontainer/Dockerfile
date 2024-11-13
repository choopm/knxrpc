FROM ubuntu:noble
ARG TARGETARCH

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y \
    curl \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# install go-task
RUN curl -sSL https://github.com/go-task/task/releases/latest/download/task_linux_$TARGETARCH.tar.gz | \
    tar zx -C /usr/local/bin/ task

# install earthly
RUN curl -sSL https://github.com/earthly/earthly/releases/latest/download/earthly-linux-$TARGETARCH -o /tmp/earthly && \
    install -m 755 /tmp/earthly /usr/bin && \
    rm /tmp/earthly

# install buf
RUN curl -sSL https://github.com/bufbuild/buf/releases/latest/download/buf-$(uname -s)-$(uname -m) -o /tmp/buf && \
    install -m 755 /tmp/buf /usr/local/bin && \
    rm /tmp/buf

LABEL org.opencontainers.image.source=https://github.com/choopm/knxrpc
