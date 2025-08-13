# Use a base image with Debian/Ubuntu for compatibility
FROM ubuntu:20.04

# Set environment variables for non-interactive installation
ENV DEBIAN_FRONTEND=noninteractive
ENV NVM_DIR=/usr/local/nvm

# Install dependencies required for Bazel, Go, Node.js, AWS CLI, Serverless Framework, and additional tools
RUN apt-get update && \
    apt-get install -y \
    curl \
    wget \
    gnupg \
    unzip \
    git \
    build-essential \
    software-properties-common \
    apt-transport-https \
    python3 \
    python3-pip \
    libssl-dev && \
    rm -rf /var/lib/apt/lists/*

# Install Bazel 5.4.0
RUN wget https://github.com/bazelbuild/bazel/releases/download/5.4.0/bazel-5.4.0-linux-x86_64 -O /usr/local/bin/bazel && \
    chmod +x /usr/local/bin/bazel && \
    bazel --version

# Install Go 1.7
RUN wget https://dl.google.com/go/go1.7.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.7.linux-amd64.tar.gz && \
    rm go1.7.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
RUN go version

# Install Node.js using nvm (Node Version Manager)
ENV NODE_VERSION=20
RUN mkdir -p $NVM_DIR && \
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.5/install.sh | bash && \
    . "$NVM_DIR/nvm.sh" && \
    nvm install $NODE_VERSION && \
    nvm alias default $NODE_VERSION && \
    ln -s "$NVM_DIR/versions/node/$(nvm current)/bin/node" /usr/local/bin/node && \
    ln -s "$NVM_DIR/versions/node/$(nvm current)/bin/npm" /usr/local/bin/npm && \
    ln -s "$NVM_DIR/versions/node/$(nvm current)/bin/npx" /usr/local/bin/npx
ENV PATH="$NVM_DIR/versions/node/$(nvm current)/bin:$PATH"
RUN . "$NVM_DIR/nvm.sh" && npm install -g npm@latest
RUN node -v && npm -v

# Install AWS CLI v2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws
RUN aws --version

# Install Serverless Framework version 3
RUN . "$NVM_DIR/nvm.sh" && \
    npm install -g serverless@3 --unsafe-perm --verbose && \
    echo "Serverless installed to: $(which serverless)" && \
    serverless --version

# Set default workdir inside the container
WORKDIR /workspace

# Default command to keep the container running
CMD ["bash"]