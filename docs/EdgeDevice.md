# `EdgeDevice` CRD

__NOTE:__ The `EdgeDevice` CRD is still in alpha stage, and changes to it may not be reflected in this doc accordingly

- [Metadata](#metadata)
- [Spec Overview](#spec-overview)
  - [`spec.connectivity`: In cloud connectivity to reach your EdgeDevice](#specconnectivity-in-cloud-connectivity-to-reach-your-edgedevice)
  - [`spec.node`: Kubernetes node resource customization](#specnode-kubernetes-node-resource-customization)
  - [`spec.network`: Network mesh for your cluster nodes and devices](#specnetwork-network-mesh-for-your-cluster-nodes-and-devices)
  - [`spec.storage`: Remote CSI for your EdgeDevices](#specstorage-remote-csi-for-your-edgedevices)
  - [`spec.pod`: Kubernetes pod related customization](#specpod-kubernetes-pod-related-customization)
  - [`spec.metricsReporters`: Optional metrics push client for your peripherals (experimental)](#specmetricsreporters-optional-metrics-push-client-for-your-peripherals-experimental)
  - [`spec.peripherals`: Peripherals definition and operation (experimental)](#specperipherals-peripherals-definition-and-operation-experimental)
- [Status](#status)

## Metadata

```yaml
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: example-edge-device
  # EdgeDevice is always namespaced, but you have to make sure the name is unique
  # across your cluster
  namespace: edge
```

## Spec Overview

__NOTE:__ if all you want is to use EdgeDevice for host management, you only need to specify `spec.connectivity`

```yaml
spec:
  connectivity: {}
  node: {}
  network: {}
  storage: {}
  pod: {}
  peripherals: []
  metricsReporters: []
```

### `spec.connectivity`: In cloud connectivity to reach your EdgeDevice

```yaml
spec:
  connectivity:
    # set method to use different connectivity config
    #
    # - grcp (requires spec.connectivity.grpc)
    #   A gRPC server will be created and served by aranya according to the
    #   spec.connectivity.grpcConfig, aranya also maintains an according
    #   service object for that server.
    #
    # - mqtt (requires spec.connectivity.mqtt)
    #   aranya will try to talk to your mqtt message broker according to the
    #   spec.connectivity.mqtt. the config option topicNamespace must
    #   match between aranya and arhat to get arhat to communicate
    #
    # - amqp (AMQP 0.9, requires spec.connectivity.amqp)
    #   aranya will connect to your AMQP 0.9 broker, typically a RabbitMQ
    #   endpoint. Like mqtt, it has an config option topicNamespace, but it is
    #   only intended to be used inside the AMQP 0.9 broker only, you have to
    #   configure your broker to expose mqtt connectivity support
    #
    # - azure-iot-hub (requires spec.connectivity.azureIoTHub)
    #   aranya will connect to iot hub endpoint for message receiving and event
    #   hub to send commands to arhat
    #
    # - gcp-iot-core (requires spec.connectivity.gcpIoTCore)
    #   aranya will receiving messages from PubSub service and publish commands
    #   via cloud iot http endpoint
    #
    method: grpc
    timers:
      # force close unary session in server after
      unarySessionTimeout: 10s

    backoff:
      initialDelay: 1s
      maxDelay: 30s
      factor: 1.5

    # --------------------------------------- #
    # config for specific connectivity method #
    # --------------------------------------- #

    gcpIoTCore:
      projectID: foo-12345
      pubSub:
        # the topic id of pubsub for iot core messages
        telemetryTopicID: default
        # (optional) default to {telemetryTopicID} if not set
        stateTopicID: default
        credentialsSecretKeyRef:
          name: my-gcp-iot-core-secret
          key: pub-sub-creds.json
      cloudIoT:
        deviceStatusPollInterval: 5m
        region: europe-west1
        registryID: bar
        deviceID: my-device
        credentialsSecretKeyRef:
          name: my-gcp-iot-core-secret
          key: cloud-iot-creds.json

    # --------------------------------------- #
    azureIoTHub:
      # the device id in your iot hub
      deviceID: foo
      iotHub:
        deviceStatusPollInterval: 5m
        connectionStringSecretKeyRef:
          name: my-azure-iot-hub-secret
          key: my-iot-hub-connection-string-key
      eventHub:
        connectionStringSecretKeyRef:
          name: my-azure-iot-hub-secret
          key: my-event-hub-connection-string-key

        # optional, will default to "$Default" if not set
        consumerGroup: "$Default"

    # --------------------------------------- #
    grpc:
      # tls certificate for grpc server
      # otherwise, aranya will setup an insecure grpc server
      tlsSecretRef: {}

    # --------------------------------------- #
    mqtt:
      # mqtt message namespace for aranya communication
      topicNamespace: example.com/example-edge-device
      # tell aranya how to connect mqtt broker
      # set secret ref to use tls when connecting to broker
      tlsSecretRef:
        # kubernetes secret name for tls connection
        name: mqtt-tls-secret
      # mqtt broker address
      broker: mqtt:1883
      # mqtt version, currently only 3.1.1 supported
      version: "3.1.1"
      # mqtt transport protocol, one of [tcp, websocket]
      transport: tcp
      userPassRef:
        # kubernetes secret name for username and password
        name: mqtt-user-pass
      clientID: aranya(example-edge-device)
      # time in seconds
      keepalive: 30

    # --------------------------------------- #
    amqp:
      # amqp broker address
      broker: rabbitmq:5673
      exchange: exchange.example.com
      # amqp vhost
      vhost: ""
      # amqp topic namespace for aranya communication
      topicNamespace: example.com.example-edge-device
      # tell aranya how to connect mqtt broker
      # set secret ref to use tls when connecting to broker
      tlsSecretRef:
        # kubernetes secret name
        name: amqp-tls-secret
      userPassRef:
        # kubernetes secret name for username and password in amqp connection
        name: amqp-user-pass
```

### `spec.node`: Kubernetes node resource customization

```yaml
spec:
  node:
    timers:
      forceSyncInterval: 10m
    # customize your node certificate CSR with following info
    cert:
      # two letter country code, ref: https://www.digicert.com/ssl-certificate-country-codes.htm
      country: ""
      state: ""
      locality: ""
      org: myOrg
      orgUnit: myOrgUnit

    # define pod cidr for this node, or it will be allocated by kubernetes dynamically
    podCIDR: 10.0.10.0/24

    # additional labels applied to the according node object
    labels: {}
      # key: value

    # additional annotations applied to the according node object
    annotations: {}
      # key: value

    taints:
    - key: example.com/taint-name
      value: foo
      effect: NoSchedule

    # node metrics collection configuration for node_exporter/windows_exporter
    metrics:
      # report metrics if it's required by the cloud (prometheus server configured to scrape node_exporter metrics)
      enabled: true

      # what metrics to collect (overrides aranya's os specific presets)
      #
      # supported metrics collectors are listed in
      #   https://github.com/prometheus/node_exporter#collectors (unix-like) and
      #   https://github.com/prometheus-community/windows_exporter#collectors (windows)
      collect:
      - time
      - textfile
      # ...

      # additional args for metrics collection
      extraArgs:
      # - --collector.
      # - --collector.service.services-where="Name='windows_exporter'"

    # field hook to update annotation/label value according
    # to current value of the Node object when node get updated
    fieldHooks:
      # text query in jq syntax
    - query: .metadata.annotations."example.com/foo"
      # kubernetes field path to apply resolved value
      targetFieldPath: metadata.labels['example.com/bar']
      # explicit value to set target field
      value: "true"
      # jq syntax expression to process query result and set target field
      #valueExpression: .[0] < 5

    rbac:
      clusterRolePermissions:
        # key: cluster role name
        foo:
          nodeVerbs:
          - get
          - list
          statusVerbs:
          - get
          - list
```

### `spec.network`: Network mesh for your cluster nodes and devices

```yaml
spec:
  network:
    # enabled network mesh
    enabled: false
```

__HINT:__ You can apply network policies to isolate the EdgeDevice workload namespace for certain group of devices to achieve namespaced network mesh.

### `spec.storage`: Remote CSI for your EdgeDevices

```yaml
spec:
  storage:
    # enabled remote CSI storage
    enabled: true
```

### `spec.pod`: Kubernetes pod related customization

```yaml
spec:
  pod:
    timers:
      forceSyncInterval: 10m

    ipv4CIDR: 10.0.10.0/24
    ipv6CIDR: ::1/128
    # dns config to override aranya's config
    dns:
      servers:
      - "1.1.1.1"
      - "8.8.8.8"
      searches:
      - cluster.local
      options:
      - ndots:5

    # how many pods this EdgeDevice node can admit, will affect kubernetes node resource
    # reporting
    allocatable: 10

    rbac:
      rolePermissions:
        foo:
          podVerbs:
          - get
          - list
          statusVerbs:
          - get
          - list
          allowExec: false
          allowAttach: false
          allowPortForward: false
          allowLog: false
      virtualpodRolePermissions:
        bar:
          podVerbs:
          - get
          - list
          statusVerbs:
          - get
          - list
          allowExec: false
          allowAttach: false
          allowPortForward: false
          allowLog: false
```

### `spec.metricsReporters`: Optional metrics push client for your peripherals (experimental)

```yaml
spec:
  metricsReporters:
  - name: foo_reporter
    connector:
      method: mqtt
      target: mqtt.example.com:1883
      # params are resolved by your custom peripheral extension
      params:
        # map<string, string>
        client_id: foo
```

### `spec.peripherals`: Peripherals definition and operation (experimental)

```yaml
spec:
  peripherals:
  - name: foo-serial
    connector:
      method: serial
      target: /dev/cu.usbserial-XXXX
      # params are resolved by your custom peripheral extension
      params:
        # map<string, string>
        baud_rate: "115200"
        data_bits: "8"
        parity: "none"
        stop_bits: "1"
    operations:
    - name: Start
      pseudoCommand: start
      # operation params are resolved by your custom peripheral extension
      params:
        text_data: FOO AT CMD
    - name: Stop
      pseudoCommand: stop
      # operation params are resolved by your custom peripheral extension
      params:
        hex_data: 0123456789ABCDEF
    metrics:
    - name: foo_node_stats
      valueType: counter
      # upload metrics along with node metrics (default)
      reportMethod: WithNodeMetrics
      # params are resolved by your custom peripheral extension
      params:
        AT: COLLECT FOO METRICS
    - name: foo_standalone_stats
      valueType: gauge
      reportMethod: WithReporter
      reporterName: foo_reporter
      # params are resolved by your custom peripheral extension
      params:
        AT: COLLECT METRICS CMD
      # params are resolved by your custom peripheral extension
      reporterParams:
        pub_topic: "/test/data"
        pub_qos: "1"
```

## Status

```yaml
status:
  hostNode: <node-name>
  network:
    podCIDRv4: ""
    podCIDRv6: ""
    meshIPv4Addr: ""
    meshIPv6Addr: ""
```
