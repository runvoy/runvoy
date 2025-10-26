# mycli Executor Container
# This container provides a complete environment for running infrastructure-as-code
# tools and other CLI commands with Git repository support.

FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install base tools and dependencies
RUN apt-get update && apt-get install -y \
    # Core utilities
    ca-certificates \
    curl \
    wget \
    git \
    openssh-client \
    unzip \
    jq \
    vim \
    # Build tools (for some CLI tools)
    build-essential \
    # Python and pip
    python3 \
    python3-pip \
    # AWS CLI dependencies
    groff \
    less \
    && rm -rf /var/lib/apt/lists/*

# Install AWS CLI v2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf aws awscliv2.zip

# Install Terraform
ARG TERRAFORM_VERSION=1.6.6
RUN curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" -o terraform.zip \
    && unzip terraform.zip \
    && mv terraform /usr/local/bin/ \
    && rm terraform.zip \
    && terraform version

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
    && rm kubectl \
    && kubectl version --client

# Install Helm
RUN curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Install Ansible
RUN pip3 install --no-cache-dir ansible

# Install additional Python tools
RUN pip3 install --no-cache-dir \
    boto3 \
    pyyaml \
    requests

# Configure Git
RUN git config --global user.email "mycli@executor" \
    && git config --global user.name "mycli Executor" \
    && git config --global init.defaultBranch main

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Set working directory
WORKDIR /workspace

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
