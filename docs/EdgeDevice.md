# `EdgeDevice` CRD

- [Metadata](#metadata)
- [Spec](#spec)
  - [Spec for related kubernetes node](#spec-for-related-kubernetes-node)
  - [Spec for kubernetes pods assigned to related node](#spec-for-kubernetes-pods-assigned-to-related-node)
  - [Spec for assiciated devices](#spec-for-assiciated-devices)
  - [Spec for cloud connectivity](#spec-for-cloud-connectivity)
- [Status](#status)
- [Appendix.A: List of supported collectors](#appendixa-list-of-supported-collectors)

## Metadata

```yaml
---
# An example edge device accessing via gRPC
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: example-edge-device
```

## Spec

```yaml
spec:
  node: {}
  pod: {}
  devices: {}
  connectivity: {}
```

### Spec for related kubernetes node

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
    # labels applied to the according node object
    labels: {}
      # key: value
    # annotations applied to the according node object
    annotations: {}
      # key: value
    storage:
      enabled: true

    # node metrics (embedded prometheus node_exporter)
    metrics:
      # report metrics if it's required by the cloud
      enabled: true
      paths:
        # sysfs path
        sysfs: /sys
        # procfs path
        procfs: /proc
        # textfile directory to store all textfile metrics
        textfile: /tmp/metrics/textfile

      # what metrics to collect
      #
      # supported metrics collectors is the subset of the collectors listed in
      #   https://github.com/prometheus/node_exporter#collectors (linux,darwin) and
      #   https://github.com/prometheus-community/windows_exporter#collectors (windows)
      # to be specific, collectors require cgo are not supported
      #
      # unsupported collectors are:
      #   1) on linux: (none)
      #   2) on darwin: boottime,meminfo,diskstats,filesystem,loadavg
      #   3) on windows: (none)
      collect:
      - time
      - textfile
      # ...

      # additional args for metrics collection, os dependent
      extraArgs:
      # example for unix-like (node_exporter)
      # - --collector.
      # example for windows (wmi_exporter)
      # - --collector.service.services-where="Name='wmi_exporter'"

    fieldHooks:
    # field hook to update annotation/label value according
    # to current value of some annotation/label
    - # query in jq syntax
      query: .metadata.annotations."example.com/foo"
      # kubernetes field path
      targetFieldPath: metadata.labels['example.com/bar']
      # value to set
      value: "true"
      # jq syntax expression to process result
      #
      # valueExpression: .[0] < 5
```

Please refer to [Appendix.A](#appendixa-list-of-supported-collectors) for the full list of supported collectors

### Spec for kubernetes pods assigned to related node

```yaml
spec:
  pod:
    timers:
      forceSyncInterval: 10m

    ipv4CIDR: 10.0.10.0/24
    ipv6CIDR: ::1/128
    # dns config to override aranya's defaults
    dns:
      servers:
      - "1.1.1.1"
      - "8.8.8.8"
      searches:
      - cluster.local
      options:
      - ndots:5
```

### Spec for assiciated devices

```yaml
spec:
  devices:
  - name: foo-serial
    connectivity:
      transport: serial
      mode: client
      target: /dev/cu.usbserial-XXXX
      params:
        baud_rate: "115200"
        data_bits: "8"
        parity: "none"
        stop_bits: "1"
    uploadConnectivity:
      transport: mqtt3.1.1/tcp
      target: mqtt.example.com:8883
      params:
        tls_insecure_skip_verify: "true"
    operations:
    - name: Start
      pseudoCommand: start
      transportParams:
        text_data: "FOO AT CMD"
    - name: Stop
      pseudoCommand: stop
      transportParams:
        hex_data: "0123456789ABCDEF"
    metrics:
    - name: foo_stats
      uploadMethod: WithStandaloneClient
      transportParams:
        text_data: "COLLECT METRICS CMD"
      uploadParams:
        pub_topic: "/test/data"
        pub_qos: "1"
```

### Spec for cloud connectivity

```yaml
spec:
  connectivity:
    # set method to use different connectivity config
    #
    # - grcp (requires grpc)
    #   A gRPC server will be created and served by aranya according to the
    #   spec.connectivity.grpcConfig, aranya also maintains an according
    #   service object for that server.
    #
    # - mqtt (requires mqtt)
    #   aranya will try to talk to your mqtt message broker according to the
    #   spec.connectivity.mqttConfig. the config option topicNamespace must
    #   match between aranya and arhat to get arhat able to communicate with
    #   aranya.
    #
    # - amqp (AMQP 0.9, requires amqp)
    #   aranya will connect to your AMQP 0.9 broker, typically a RabbitMQ
    #   endpoint. Like mqtt, it has an config option topicNamespace, but it is
    #   only intended to be used inside the AMQP 0.9 broker only, you have to
    #   configure your broker to expose mqtt connectivity support
    #
    # - azure-iot-hub (requires azureIoTHub)
    #   aranya will connect to iot hub endpoint for message receiving and event
    #   hub to send commands to arhat
    #
    # - gcp-iot-core (requires gcpIoTCore)
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
      # here, with empty ref, aranya will setup an insecure grpc server
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

## Status

```yaml
status:
  hostNode: <node-name>
```

## Appendix.A: List of supported collectors

1. Linux:
   - `arp`
   - `bcache`
   - `bonding`
   - `conntrack`
   - `cpu`
   - `cpufreq`
   - `diskstats`
   - `edac`
   - `entropy`
   - `filefd`
   - `filesystem`
   - `hwmon`
   - `infiniband`
   - `ipvs`
   - `loadavg`
   - `mdadm`
   - `meminfo`
   - `netclass`
   - `netdev`
   - `netstat`
   - `nfs`
   - `nfsd`
   - `pressure`
   - `sockstat`
   - `stat`
   - `textfile`
   - `time`
   - `timex`
   - `uname`
   - `vmstat`
   - `xfs`
   - `zfs`
   - `buddyinfo`
   - `drbd`
   - `interrupts`
   - `ksmd`
   - `logind`
   - `meminfo_numa`
   - `mountstats`
   - `ntp`
   - `processes`
   - `qdisc`
   - `runit`
   - `supervisord`
   - `systemd`
   - `tcpstat`
   - `wifi`
   - `perf`

2. Darwin:
   - `cpu`
   - `netdev`
   - `uname`
   - `textfile`

3. Windows:
   - `ad`
   - `adfs`
   - `cpu`
   - `cs`
   - `container`
   - `dns`
   - `hyperv`
   - `iis`
   - `logical_disk`
   - `logon`
   - `memory`
   - `msmq`
   - `mssql`
   - `netframework_clrexceptions`
   - `netframework_clrinterop`
   - `netframework_clrjit`
   - `netframework_clrloading`
   - `netframework_clrlocksandthreads`
   - `netframework_clrmemory`
   - `netframework_clrremoting`
   - `netframework_clrsecurity`
   - `net`
   - `os`
   - `process`
   - `service`
   - `system`
   - `tcp`
   - `thermalzone`
   - `textfile`
   - `vmware`
