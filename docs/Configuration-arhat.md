# `arhat` Config

- [Overview](#overview)
- [Config](#config)
  - [Section `arhat`](#section-arhat)
  - [Section `runtime`](#section-runtime)
  - [Section `storage`](#section-storage)
  - [Section `connectivity`](#section-connectivity)
- [Appendix.A: List of supported cipher suites](#appendixa-list-of-supported-cipher-suites)

## Overview

The configuration of `arhat` is defined in a `yaml` file

## Config

The `arhat` configuration file contains four major sections (`arhat`, `runtime`, `storage` and `connectivity`):

```yaml
arhat:
  # ...
runtime:
  # ...
storage:
  # ...
connectivity:
  # ...
```

Each section is used to define a specific aspect of `arhat`'s behavior

__NOTE:__ You can include environment variables (`$FOO` or `${FOO}`) in the config file, they will be expanded when `arhat` read it.

### Section `arhat`

This section defines `arhat`'s behavior

```yaml
arhat:
  # log options, you can designate mutiple destination as you wish
  log:
    # log level
    #
    # can be one of the following:
    #   - debug
    #   - info
    #   - error
    #   - silent  (no log)
  - level: debug
    # log output format
    #
    # value can be one of the following:
    #   - console
    #   - json
    format: console
    # log output destination file
    #
    # value can be one of the following:
    #   - stdout
    #   - stderr
    #   - {FILE_PATH}
    file: stderr
    # whether expose this log file for `kubectl logs`
    #
    # if true, `kubectl logs` to the according virtual pod will
    # use this file as log source
    #
    # if multiple log config section has `kubeLog: true`, the
    # last one in the list will be used
    kubeLog: false
  - level: debug
    format: console
    kubeLog: true
    file: /var/log/arhat.log

  # host operations
  # for security reason, all of them are set to `false` by defualt
  host:
    # allow `kubectl exec/cp` to device host
    allowExec: true
    # allow `kubectl attach` to device host
    allowAttach: true
    # allow `kubectl port-forward` to device host
    allowPortForward: true
    # allow `kubectl logs` to view arhat log file exposed with `kubeLog: true`
    allowLog: true

  # kubernetes node operation
  node:
    # extInfo a list of key value pairs to set extra node related values
    # the default value is
    # - value: ""
    #   operator: =
    #   valueType: string
    #   applyTo: ""
    extInfo:
    - # value must be a string, no matter what type it is
      value: "1"
      # operator available: [=, +=, -=]
      operator: +=
      # valueType available: [string, int, float]
      valueType: int
      # applyTo which node object field
      # value available: [metadata.annotations[''], metadata.labels['']]
      applyTo: metadata.annotations['example.com/key']

  # application optimization
  optimization:
    # set GOMAXPROCS
    maxProcessors: 1
    pprof:
      # enable pprof
      enabled: false
      # pprof listen address
      listen: localhost:8080
      # http base path to replace `/debug/pprof`
      httpPath: /foo
      # parameter for `runtime.SetMutexProfileFraction(int)`
      mutexProfileFraction: 100
      # parameter for `runtime.SetBlockProfileRate(int)`
      blockProfileRate: 1
```

### Section `runtime`

This section defines the container runtime configuration

__NOTE:__ this section is ignored by `arhat-none`, which has no container runtime support

```yaml
runtime:
  # enable bundled container runtime
  #
  # default: true
  #
  # if set to false, will create a empty no-op runtime
  enabled: true

  # the directory to store pod data and container specific files
  #
  # generally it will include:
  #   - volume data from configmap/secret
  #   - directory created by emptyDir volume
  #   - container resolv.conf
  dataDir: /var/lib/arhat

  # managementNamespace is the container runtime's namespace
  #
  # used in:
  #   arhat-libpod
  # ignored in:
  #   arhat-docker
  managementNamespace: container.arhat.dev

  # pauseImage is the image used to create the first container in pod
  # to claim all the linux namespaces required by the pod
  pauseImage: k8s.gcr.io/pause:3.1

  # pauseCommand is the command to run pause image
  pauseCommand: /pause

  # service endpoints of the container runtime
  endpoints:

    # image endpoint (image management)
    image:
      # the url to connect the image endpoint
      #
      # used in:
      #   - arhat-docker
      # ignored in:
      #   - arhat-libpod
      endpoint: unix:///var/run/docker.sock

      # timeout when dial image endpoint
      #
      # used in:
      #   - arhat-docker
      # ignored in:
      #   - arhat-libpod
      dialTimeout: 10s

      # timeout when working on image related job (image pull and lookup)
      actionTimeout: 2m

    # runtime endpoint (container management)
    runtime:
      # the url to connect the runtime endpoint
      #
      # used in:
      #   - arhat-docker
      # ignored in:
      #   - arhat-libpod
      endpoint: unix:///var/run/docker.sock

      # timeout when dial runtime endpoint
      #
      # used in:
      #   - arhat-docker
      # ignored in:
      #   - arhat-libpod
      dialTimeout: 10s

      # timeout when working on container related job
      actionTimeout: 2m
```

### Section `storage`

This section defines how to mount remote volumes

```yaml
storage:
  # backend used to mount remote volume
  #
  # value can be one of the following
  #   - "none" or "" (disabled)
  #   - sshfs
  backend: sshfs

  # stdout file for backend's mount program
  #
  # value can be one of the following
  #   - none (discard any output)
  #   - stdout (to arhat's stdout)
  #   - stderr (to arhat's stderr)
  #   - /path/to/file (some file to store output)
  stdoutFile: stdout

  # stderr file for backend's mount program (same as `stdoutFile`, but for stderr)
  stderrFile: stderr

  # treat command execution successful after this time period
  processCheckTimeout: 5s

  # lookupPaths are paths to lookup required executables in addition
  # to paths in $PATH env
  lookupPaths:
  - /opt/bin

  # args for each executable used in backend (map[string][]string)
  args:
    # for backend `sshfs`, the executable `sshfs` is used
    # you MUST specify it's args with env ref like the following example
    #
    # more specifically:
    #   - 1st arg MUST be `root@HOST:${ARHAT_STORAGE_REMOTE_PATH}`
    #       `HOST` is the address to connect the aranya sftp server
    #   - 2nd arg MUST be `${ARHAT_STORAGE_LOCAL_PATH}`
    #   - all the following args can be defined up to your use case
    sshfs:
    - "root@example.com:${ARHAT_STORAGE_REMOTE_PATH}"
    - ${ARHAT_STORAGE_LOCAL_PATH}
    - -p
    - "54322"
    - -o
    - sshfs_debug
```

### Section `connectivity`

This section defines how to connect to the server/broker and finally communicate with `aranya`

```yaml
connectivity:
  # dial timeout
  dialTimeout: 10s

  # initial backoff duration when reconnecting to server/broker
  initialBackoff: 10s

  # max backoff duration when reconnecting to server/broker
  maxBackoff: 3m

  # factor used to multiply backoff duration
  #
  # e.g.
  #   this backoff duration is 10s, if the backoffFactor is 1.5
  #   then the next backoff duration is 15s
  backoffFactor: 1.5

  # connectivity method to use
  #
  # value can be one of the following:
  #   - mqtt
  #   - grpc
  #   - coap
  #   - auto (select by priority, fallback if failed)
  method: mqtt

  grpc:
    # grpc server address
    endpoint: grpc.example.com

    # priority of grpc connectivity, used to determine connection priority when method is auto, defaults to 0
    priority: 1

    # TLS settings for grpc connection
    #
    # fields in this section are identical to those in
    # `connectivity.mqttConfig.tls`
    tls:
      enabled: true

  coap:
    # coap broker address with port
    endpoint: coap.example.com:5684

    # priority of coap connectivity, used to determine connection priority when method is auto, defaults to 0
    priority: 1

    # transport protocol
    #
    # value can be one of the following
    #   - udp{,4,6} (defaults to udp)
    #   - tcp{,4,6}
    transport: udp

    # custom path namespace for coap uri-path options
    # usually coap brokers are integrated into mqtt broker
    # so it's like a mqtt topic namespace but with some
    # server specific prefix (ususally `/ps/`)
    pathNamespace: /ps/exmaple/topic/foo

    # custom string key value pair for coap uri-query options
    uriQueries:
      foo: bar
      a: b

    # TLS/DTLS settings for tcp/udp transport
    #
    # most fields in this section are identical to those in
    # `connectivity.mqttConfig.tls` with exceptions noted here
    tls:
      enabled: true

      # allow insecure hash functions
      allowInsecureHashes: false

      # use dtls with pre shared key
      preSharedKey:
        # map server hint(s) to pre shared key(s)
        # column separated base64 encoded key value pairs
        serverHintMapping:
        # empty key to match all server hint
        - :dGhpcyBpcyBteSBwc2s= # `this is my psk`
        - a2V5:dmFsdWU=
        # the client hint provided to server, base64 encoded value
        identityHint: aWRlbnRpdHkgaGludA== # `identity hint`

  mqtt:
    # variant of mqtt protocol
    #
    # value can be one of the following
    #   - standard (default)
    #   - azure-iot-hub
    #   - gcp-iot-core
    #   - aws-iot-core
    variant: aws-iot-core

    # mqtt broker address
    #
    # in the form of {ADDRESS}:{PORT}
    endpoint: mqtt.example.com:8883

    # priority of mqtt connectivity, used to determine connection priority when method is auto, defaults to 0
    priority: 1

    # transport protocol
    #
    # value can be one of the following
    #   - tcp (default)
    #   - websocket
    transport: tcp

    # custom topic namespace used to pub/sub mqtt topics
    #
    # its meaning depends on the value of variant:
    #
    # for `standard` or `aws-iot-core`
    #       MUST set and following topics will be used:
    #         - ${topicNamespace}/msg
    #         - ${topicNamespace}/cmd
    #         - ${topicNamespace}/status  (will topic)
    #
    # for `gcp-iot-core`
    #       1) when it's empty, mqtt messages will go to registry's default telemetry topic
    #           see https://cloud.google.com/iot/docs/how-tos/mqtt-bridge#publishing_telemetry_events
    #       2) when it's non-empty, it MUST NOT start with a slash (`/`)
    #           its value will be treated as subfolder in gcp-iot-core
    #           see https://cloud.google.com/iot/docs/how-tos/mqtt-bridge#publishing_telemetry_events_to_additional_cloud_pubsub_topics
    #
    # for `azure-iot-hub`
    #       it is the extra property bag in the format of `{key1}={value1}&{key2}={value2}...`
    #       arhat will set property bag to `dev=${DEVICE_ID}&arhat&${topicNamespace}`
    topicNamespace: arhat.dev/aranya/foo

    # clientID
    #
    # its meaning varies when `variant` is different:
    #
    # for `standard`, it's up to your choice
    # for `azure-iot-hub`, it's the `${DEVICE_ID}`
    # for `gcp-iot-core`, it's the `projects/${PROJECT_ID}/locations/${REGION}/registries/${REGISTRY_ID}/devices/${DEVICE_ID}`
    # for `aws-iot-core`, it's up to your iot policies
    clientID: foo

    # username
    #
    # its meaning varies when `variant` is different:
    #
    # for `standard`, it's up to your choice
    # for `azure-iot-hub`, it will be generated by arhat
    # for `gcp-iot-core` or `aws-iot-core`, it's ignored
    username: foo-user

    # password
    #
    # its meaning varies when `variant` is different:
    #
    # for `standard`, it's up to your choice
    # for `azure-iot-hub`, it's the `${DEVICE_SAS_TOKEN}`
    # for `gcp-iot-core` or `aws-iot-core`, it's ignored
    password: my-password

    # tls configuration
    tls:
      # CA cert file (PEM/ASN.1 format)
      caCert: /path/to/ca.crt

      # client cert file (PEM format)
      #
      # for variant `gcp-iot-core`, this field MUST be empty
      cert: /path/to/client.crt

      # client private key file (PEM format)
      #
      # for variant `gcp-iot-core`
      #   the private key is used to sign the JWT token, and its format can also be `ASN.1`
      key:  /path/to/client.key

      # tls server name override
      serverName: foo.example.com

      # skip verify inscure server cert
      insecureSkipVerify: false

      # ONLY intended for DEBUG use
      #
      # if set, will record the random key used in the tls connection to this file
      # and the file can be used for applications like wireshark to decrypt tls connection
      keyLogFile: /path/to/tmp/tls/log

      # set cipher suites expected to ues when establishing tls connection
      #
      # please refer to [Appendix.B: List of supported cipher suites] for full
      # list of supported cipher suites
      cipherSuites:
      - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
```

## Appendix.A: List of supported cipher suites

1. TCP TLS

   - `TLS_RSA_WITH_RC4_128_SHA`
   - `TLS_RSA_WITH_3DES_EDE_CBC_SHA`
   - `TLS_RSA_WITH_AES_128_CBC_SHA`
   - `TLS_RSA_WITH_AES_256_CBC_SHA`
   - `TLS_RSA_WITH_AES_128_CBC_SHA256`
   - `TLS_RSA_WITH_AES_128_GCM_SHA256`
   - `TLS_RSA_WITH_AES_256_GCM_SHA384`
   - `TLS_ECDHE_ECDSA_WITH_RC4_128_SHA`
   - `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`
   - `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`
   - `TLS_ECDHE_RSA_WITH_RC4_128_SHA`
   - `TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA`
   - `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`
   - `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`
   - `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256`
   - `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256`
   - `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
   - `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
   - `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
   - `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
   - `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305`
   - `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305`
   - `TLS_AES_128_GCM_SHA256`
   - `TLS_AES_256_GCM_SHA384`
   - `TLS_CHACHA20_POLY1305_SHA256`
   - `TLS_FALLBACK_SCSV`

2. UDP DTLS

  - `TLS_ECDHE_ECDSA_WITH_AES_128_CCM`
  - `TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8`
  - `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
  - `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
  - `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`
  - `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`
  - `TLS_PSK_WITH_AES_128_CCM`
  - `TLS_PSK_WITH_AES_128_CCM_8`
  - `TLS_PSK_WITH_AES_128_GCM_SHA256`
