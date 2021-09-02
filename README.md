## Introduction

This repository contains tools that allow the NVIDIA runtime to be configured as one of the runtimes for Docker and containerd. 

### Docker

```bash
docker setup \
    --runtime-name NAME \
        /run/nvidia/toolkit
```

Configure the `nvidia-container-runtime` as a docker runtime named `NAME`. If the `--runtime-name` flag is not specified, this runtime would be called `nvidia`. A runtime named `nvidia-experimental` will also be configured using the `nvidia-container-runtime-experimental` OCI-compliant runtime shim.

Since `--set-as-default` is enabled by default, the specified runtime name will also be set as the default docker runtime. This can be disabled by explicityly specifying `--set-as-default=false`.

**Note**: If `--runtime-name` is specified as `nvidia-experimental` explicitly, the `nvidia-experimental` runtime will be configured as the default runtime, with the `nvidia` runtime still configured and available for use.

The following table describes the behaviour for different `--runtime-name` and `--set-as-default` flag combinations.

| Flags                                                       | Installed Runtimes              | Default Runtime       |
|-------------------------------------------------------------|:--------------------------------|:----------------------|
| **NONE SPECIFIED**                                          | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--runtime-name nvidia`                                     | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--runtime-name NAME`                                       | `NAME`, `nvidia-experimental`   | `NAME`                |
| `--runtime-name nvidia-experimental`                        | `nvidia`, `nvidia-experimental` | `nvidia-experimental` |
| `--set-as-default`                                          | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--set-as-default --runtime-name nvidia`                    | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--set-as-default --runtime-name NAME`                      | `NAME`, `nvidia-experimental`   | `NAME`                |
| `--set-as-default --runtime-name nvidia-experimental`       | `nvidia`, `nvidia-experimental` | `nvidia-experimental` |
| `--set-as-default=false`                                    | `nvidia`, `nvidia-experimental` | **NOT SET**           |
| `--set-as-default=false --runtime-name NAME`                | `NAME`, `nvidia-experimental`   | **NOT SET**           |
| `--set-as-default=false --runtime-name nvidia`              | `nvidia`, `nvidia-experimental` | **NOT SET**           |
| `--set-as-default=false --runtime-name nvidia-experimental` | `nvidia`, `nvidia-experimental` | **NOT SET**           |

These combinations also hold for the environment variables that map to the command line flags: `DOCKER_RUNTIME_NAME`, `DOCKER_SET_AS_DEFAULT`.

### Containerd

```bash
containerd setup \
    --runtime-class NAME \
        /run/nvidia/toolkit
```

Configure the `nvidia-container-runtime` as a runtime class named `NAME`. If the `--runtime-class` flag is not specified, this runtime would be called `nvidia`. A runtime class named `nvidia-experimental` will also be configured using the `nvidia-container-runtime-experimental` OCI-compliant runtime shim.

Adding the `--set-as-default` flag as follows:
```bash
containerd setup \
    --runtime-class NAME \
    --set-as-default \
        /run/nvidia/toolkit
```
will set the runtime class `NAME` (or `nvidia` if not specified) as the default runtime class.

**Note**: If `--runtime-class` is specified as `nvidia-experimental` explicitly and `--set-as-default` is specified, the `nvidia-experimental` runtime will be configured as the default runtime class, with the `nvidia` runtime class still configured and available for use.

The following table describes the behaviour for different `--runtime-class` and `--set-as-default` flag combinations.

| Flags                                                  | Installed Runtime Classes       | Default Runtime Class |
|--------------------------------------------------------|:--------------------------------|:----------------------|
| **NONE SPECIFIED**                                     | `nvidia`, `nvidia-experimental` | **NOT SET**           |
| `--runtime-class NAME`                                 | `NAME`, `nvidia-experimental`   | **NOT SET**           |
| `--runtime-class nvidia`                               | `nvidia`, `nvidia-experimental` | **NOT SET**           |
| `--runtime-class nvidia-experimental`                  | `nvidia`, `nvidia-experimental` | **NOT SET**           |
| `--set-as-default`                                     | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--set-as-default --runtime-class NAME`                | `NAME`, `nvidia-experimental`   | `NAME`                |
| `--set-as-default --runtime-class nvidia`              | `nvidia`, `nvidia-experimental` | `nvidia`              |
| `--set-as-default --runtime-class nvidia-experimental` | `nvidia`, `nvidia-experimental` | `nvidia-experimental` |

These combinations also hold for the environment variables that map to the command line flags.

---
### Running toolkit tests locally

```bash
make build-ubuntu18.04
````

To run only the toolkit tests:
```bash
export TEST_CASES=toolkit
make test-ubuntu18.04
```

To check the generated files in `shared-ubuntu18.04` (i.e. skipping cleanup)
```bash
export CLEANUP=false
make test-ubuntu18.04
```


