# libmqtt

[![Build Status](https://api.travis-ci.com/goiiot/libmqtt.svg?branch=master)](https://travis-ci.com/goiiot/libmqtt) [![GoDoc](https://godoc.org/github.com/goiiot/libmqtt?status.svg)](https://godoc.org/github.com/goiiot/libmqtt) [![GoReportCard](https://goreportcard.com/badge/goiiot/libmqtt)](https://goreportcard.com/report/github.com/goiiot/libmqtt) [![codecov](https://codecov.io/gh/goiiot/libmqtt/branch/master/graph/badge.svg)](https://codecov.io/gh/goiiot/libmqtt)

Feature rich modern MQTT library in pure Go, for `Go`, `C/C++`, `Java`

## Table of contents

- [Features](#features)
- [Usage](#usage)
- [Topic Routing](#topic-routing)
- [Session Persist](#session-persist)
- [Benchmark](#benchmark)
- [Extensions](#extensions)
- [LICENSE](#license)

## Features

1. MQTT v3.1.1/v5.0 client support (async only)
1. High performance and less memory footprint (see [Benchmark](#benchmark))
1. Customizable topic routing (see [Topic Routing](#topic-routing))
1. Multiple Builtin session persist methods (see [Session Persist](#session-persist))
1. [C/C++ lib](./c/), [Java lib](./java/), [Command line client](./cmd/) support
1. Idiomatic Go

## Usage

This package can be used as

- A [Go lib](#as-a-go-lib)
- A [C/C++ lib](#as-a-cc-lib)
- A [Java lib](#as-a-java-lib)
- A [Command line client](#as-a-command-line-client)
- [MQTT infrastructure](#as-mqtt-infrastructure)

### As a Go lib

#### Prerequisite

- Go 1.9+

#### Steps

**TL;DR:** You can find a full example at [examples/client.go](./examples/client.go)

1.Go get this project

```bash
go get github.com/goiiot/libmqtt
```

2.Import this package in your project file

```go
import "github.com/goiiot/libmqtt"
```

3.Create a custom client

```go
// Create a client and enable auto reconnect when connection lost
// We primarily use `RegexRouter` for client
client, err := libmqtt.NewClient(
    // enable keepalive (10s interval) with 20% tolerance
    libmqtt.WithKeepalive(10, 1.2),
    // enable auto reconnect and set backoff strategy
    libmqtt.WithAutoReconnect(true),
    libmqtt.WithBackoffStrategy(time.Second, 5*time.Second, 1.2),
    // use RegexRouter for topic routing if not specified
    // will use TextRouter, which will match full text
    libmqtt.WithRouter(libmqtt.NewRegexRouter()),
)

if err != nil {
    // handle client creation error
    panic("create mqtt client failed")
}
```

__Notice__: If you would like to explore all the options available, please refer to [GoDoc#Option](https://godoc.org/github.com/goiiot/libmqtt#Option)

4.Register the handlers and Connect, then you are ready to pub/sub with server

<details>
<summary>Optional, but we recommend to register handlers for pub, sub, unsub, net error and persist error, and you can gain more controllability of the lifecycle of the client</summary>
<pre><code>
client.HandlePub(PubHandler) // register handler for pub success/fail (optional, but recommended)
client.HandleSub(SubHandler) // register handler for sub success/fail (optional, but recommended)
client.HandleUnSub(UnSubHandler) // register handler for unsub success/fail (optional, but recommended)
client.HandleNet(NetHandler) // register handler for net error (optional, but recommended)
client.HandlePersist(PersistHandler) // register handler for persist error (optional, but recommended)

// define your topic handlers like a golang http server
client.Handle("foo", func(topic string, qos libmqtt.QosLevel, msg []byte) {
    // handle the topic message
})

client.Handle("bar", func(topic string, qos libmqtt.QosLevel, msg []byte) {
    // handle the topic message
})
</code></pre>
</details>

```go
// connect to server
client.ConnectServer("test.mosquitto.org:8883", 
	libmqtt.WithCustomTLS(nil),
	libmqtt.WithConnHandleFunc(func(server string, code byte, err error) {
		if err != nil {
			// failed
			panic(err)
		}
		
		if code != libmqtt.CodeSuccess {
			// server rejected or in error
			panic(code)
		}

		// success
		// you are now connected to the `server`
		// (the `server` is one of your provided `servers` when create the client)
		// start your business logic here or send a signal to your logic to start

		// subscribe some topic(s)
		client.Subscribe([]*libmqtt.Topic{
			{Name: "foo"},
			{Name: "bar", Qos: libmqtt.Qos1},
		}...)

		// publish some topic message(s)
		client.Publish([]*libmqtt.PublishPacket{
			{TopicName: "foo", Payload: []byte("bar"), Qos: libmqtt.Qos0},
			{TopicName: "bar", Payload: []byte("foo"), Qos: libmqtt.Qos1},
		}...)
	}),
)
```

5.Unsubscribe from topic(s)

```go
client.UnSubscribe("foo", "bar")
```

6.Destroy the client when you would like to

```go
// use true for a immediate disconnect to server
// use false to send a DisConn packet to server before disconnect
client.Destroy(true)
```

### As a C/C++ lib

Please refer to [c - README.md](./c/README.md)

### As a Java lib

Please refer to [java - README.md](./java/README.md)

### As a command line client

Please refer to [cmd/libmqtt - README.md](./cmd/libmqtt/README.md)

### As MQTT infrastructure

This package can also be used as MQTT packet encoder and decoder

```go
// decode one mqtt 3.1.1 packet from reader
packet, err := libmqtt.Decode(libmqtt.V311, reader)
// ...

// encode one mqtt packet to buffered writer
err := libmqtt.Encode(packet, bufferedWriter)
// ...
```

## Topic Routing

Routing topics is one of the most important thing when it comes to business logic, we currently have built two `TopicRouter`s which is ready to use, they are `TextRouter` and `RegexRouter`

- `TextRouter` will match the exact same topic which was registered to client by `Handle` method. (this is the default router in a client)
- `RegexRouter` will go through all the registered topic handlers, and use regular expression to test whether that is matched and should dispatch to the handler

If you would like to apply other routing strategy to the client, you can provide this option when creating the client

```go
client, err := libmqtt.NewClient(
    // ...
    // e.g. use `RegexRouter`
    libmqtt.WithRouter(libmqtt.NewRegexRouter()),
    // ...
)
```

## Session Persist

Per MQTT Specification, session state should be persisted and be recovered when next time connected to server without clean session flag set, currently we provide persist method as following:

1. `NonePersist` - no session persist
2. `memPersist` - in memory session persist
3. `filePersist` - files session persist (with write barrier)
4. `redisPersist` - redis session persist (available inside [github.com/goiiot/libmqtt/extension](./extension/) package)

__Note__: Use `RedisPersist` if possible.

## Benchmark

The procedure of the benchmark is:

1. Create the client
2. Connect to server
3. Publish N times to topic `foo`
4. Unsubscribe topic (just ensure all pub message has been sent)
5. Destroy client (without disconnect packet)

The benchmark result listed below was taken on a MacBook Pro 13' (Early 2015, macOS 10.13.2), statistics inside which is the value of ten times average

| Bench Name                              | Pub Count | ns/op | B/op | allocs/op |
| --------------------------------------- | --------- | ----- | ---- | --------- |
| BenchmarkLibmqttClient-4 (this project) | 100000    | 20187 | 176  | 6         |
| BenchmarkPahoClient-4    (eclipse paho) | 100000    | 25072 | 816  | 15        |

You can make the benchmark using source code from [benchmark](./benchmark/)

## Extensions

Helpful extensions for libmqtt (see [extension](./extension/))

## LICENSE

[![GitHub license](https://img.shields.io/github/license/goiiot/libmqtt.svg)](https://github.com/goiiot/libmqtt/blob/master/LICENSE.txt)

```text
Copyright Go-IIoT (https://github.com/goiiot)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
