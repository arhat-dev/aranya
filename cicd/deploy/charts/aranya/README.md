# aranya

`aranya` is a controller for all kinds of devices

## Introduction

This is the official helm chart for [aranya](https://github.com/arhat-dev/aranya), you can deploy `aranya` to your clusters to manage any devices as kubernetes node via message queue or grpc

## Prerequisites

- `helm` v3
- `Kubernetes` 1.14+

## Installing the Chart

```bash
helm install my-release arhat-dev/aranya
```

## Uninstalling the Chart

```bash
helm delete my-release
```

## Configuration

Please refer to the [`values.yaml`](https://github.com/arhat-dev/aranya/blob/master/cicd/deploy/charts/aranya/values.yaml)
