# NVIDIA Container Toolkit Container

[![GitHub license](https://img.shields.io/github/license/NVIDIA/container-config?style=flat-square)](https://raw.githubusercontent.com/NVIDIA/container-config/main/LICENSE)
[![Documentation](https://img.shields.io/badge/documentation-wiki-blue.svg?style=flat-square)](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/index.html)

## Introduction

This repository implements the NVIDIA Container Toolkit Container that is used
in the context of the the [GPU Operator](https://github.com/NVIDIA/gpu-operator)
to configure container runtimes such as Containerd and Cri-o to allow access to
GPUs in Kubernetes clusters.

For some time this functionality was migrated to the [NVIDIA Container Toolkit repository](https://gitlab.com/nvidia/container-toolkit/container-toolkit),
but has been pulled out again to separate the concerns around producing container images.

## Issues and Contributing

[Checkout the Contributing document!](CONTRIBUTING.md)

* Please let us know by [filing a new issue](https://github.com/NVIDIA/container-config/issues/new)
* You can contribute by creating a [pull request](https://github.com/NVIDIA/container-config/compare) to our public GitHub repository
