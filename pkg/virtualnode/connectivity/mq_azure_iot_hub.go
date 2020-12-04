/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package connectivity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/log"
	"github.com/Azure/azure-amqp-common-go/v3/cbs"
	"github.com/Azure/azure-amqp-common-go/v3/conn"
	"github.com/Azure/azure-amqp-common-go/v3/sas"
	"github.com/Azure/azure-amqp-common-go/v3/uuid"
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	amqp "github.com/Azure/go-amqp"
)

func newAzureIoTHubClient(parent *MessageQueueManager) (*azureIoTHubClient, error) {
	opts := parent.config.AzureIoTHubOpts
	recvOpts := []eventhub.ReceiveOption{
		eventhub.ReceiveWithLatestOffset(),
	}
	if cg := opts.Config.EventHub.ConsumerGroup; cg != "" {
		recvOpts = append(recvOpts, eventhub.ReceiveWithConsumerGroup(cg))
	}

	parsed, err := parseIoTHubConnectionString(opts.IoTHubConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iot hub connection string: %w", err)
	}

	tp, err := sas.NewTokenProvider(sas.TokenProviderWithKey(parsed.KeyName, parsed.Key))
	if err != nil {
		return nil, fmt.Errorf("failed to create iot hub token provider: %w", err)
	}

	pollInterval := opts.Config.IoTHub.DeviceStatusPollInterval.Duration
	if pollInterval == 0 {
		pollInterval = time.Minute
	}

	clientCtx, exit := context.WithCancel(parent.ctx)
	client := &azureIoTHubClient{
		log:  parent.log,
		ctx:  clientCtx,
		exit: exit,

		deviceID:            opts.Config.DeviceID,
		pollInterval:        pollInterval,
		eventHubConnStr:     opts.EventHubConnectionString,
		iotHubParsedConn:    parsed,
		iotHubTokenProvider: tp,

		onRecvMsg: parent.onRecvMsg,
		rejectAgent: func() {
			parent.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "iot hub connection lost")
		},
		onlineMsg:  nil,
		offlineMsg: nil,

		recvOpts:        recvOpts,
		senderStore:     new(atomic.Value),
		senderClosedSig: make(chan struct{}),

		msgPropTo: fmt.Sprintf("/devices/%s/messages/devicebound", opts.Config.DeviceID),
	}

	client.onlineMsg, client.offlineMsg = createStateMessages(opts.Config.DeviceID)

	return client, nil
}

type azureIoTHubClient struct {
	log  log.Interface
	ctx  context.Context
	exit context.CancelFunc

	deviceID            string
	pollInterval        time.Duration
	eventHubConnStr     string
	iotHubParsedConn    *conn.ParsedConn
	iotHubTokenProvider *sas.TokenProvider

	onRecvMsg   func(data []byte)
	rejectAgent func()
	onlineMsg   []byte
	offlineMsg  []byte

	hubOpts           []eventhub.HubOption
	recvOpts          []eventhub.ReceiveOption
	senderStore       *atomic.Value
	senderClosedSig   chan struct{}
	connectedToIoTHub uint32

	msgPropTo string
}

func (c *azureIoTHubClient) getSender() *amqp.Sender {
	if v := c.senderStore.Load(); v != nil {
		return v.(*amqp.Sender)
	}

	return nil
}

func (c *azureIoTHubClient) PubTopics() []string {
	return []string{fmt.Sprintf("/messages/devicebound?To=%s", c.msgPropTo)}
}

func (c *azureIoTHubClient) SubTopics() []string {
	return []string{}
}

// iot hub amqp connection is used to send cmd to edge device
func (c *azureIoTHubClient) createSender() (_ *amqp.Sender, close func(), err error) {
	amqpClient, err := amqp.Dial(fmt.Sprintf("amqps://%s", c.iotHubParsedConn.Host), amqp.ConnSASLAnonymous())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create iot hub amqp client: %w", err)
	}

	err = cbs.NegotiateClaim(c.ctx, c.iotHubParsedConn.Host, amqpClient, c.iotHubTokenProvider)
	if err != nil {
		return nil, nil, err
	}

	session, err := amqpClient.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	sender, err := session.NewSender(
		amqp.LinkSenderSettle(amqp.ModeMixed),
		amqp.LinkReceiverSettle(amqp.ModeFirst),
		amqp.LinkTargetAddress("/messages/devicebound"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create sender: %w", err)
	}

	return sender, func() {
		_ = sender.Close(c.ctx)
		_ = session.Close(c.ctx)
		_ = amqpClient.Close()
	}, nil
}

// event hub is used to receive telemetry messages sent from devices
// it will listen events from all partitions
func (c *azureIoTHubClient) createEventHubClient() (connCtx context.Context, hub *eventhub.Hub, err error) {
	hub, err = eventhub.NewHubFromConnectionString(c.eventHubConnStr, c.hubOpts...)
	if err != nil {
		return nil, nil, err
	}

	connCtx, cancelConn := context.WithCancel(c.ctx)
	defer func() {
		if err != nil {
			cancelConn()
		}
	}()

	info, err := hub.GetRuntimeInformation(connCtx)
	if err != nil {
		return nil, nil, err
	}

	// listen to all partitions
	for _, pID := range info.PartitionIDs {
		h, err := hub.Receive(connCtx, pID, c.handleEvent, c.recvOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to receive from event hub: %w", err)
		}

		go func(h *eventhub.ListenerHandle) {
			defer func() {
				_ = h.Close(connCtx)
			}()

			select {
			case <-h.Done():
				c.log.D("event hub event listener exited", log.Error(h.Err()))
				cancelConn()
				return
			case <-connCtx.Done():
				return
			}
		}(h)
	}

	return connCtx, hub, nil
}

func (c *azureIoTHubClient) Connect() (err error) {
	connCtx, hub, err := c.createEventHubClient()
	if err != nil {
		return fmt.Errorf("failed to create event hub client: %w", err)
	}
	defer func() {
		if err != nil {
			err2 := hub.Close(c.ctx)
			if err2 != nil {
				c.log.D("failed to close hub when error happened", log.Error(err2))
			}
		}
	}()

	sender, closeSender, err := c.createSender()
	if err != nil {
		return fmt.Errorf("failed to create iot hub sender: %w", err)
	}

	// check device connection since azure-iot-hub won't retain will message
	deviceConnected, err := c.checkDeviceConnected()
	if err != nil {
		return fmt.Errorf("failed to check device twin status: %w", err)
	}

	if deviceConnected {
		c.onRecvMsg(c.onlineMsg)
	} else {
		c.onRecvMsg(c.offlineMsg)
	}

	atomic.StoreUint32(&c.connectedToIoTHub, 1)
	c.senderStore.Store(sender)

	// handle reconnect in new goroutine
	go func() {

		senderRefreshTk := time.NewTicker(time.Hour)
		deviceStatusCheckTk := time.NewTicker(c.pollInterval)
		defer func() {
			atomic.StoreUint32(&c.connectedToIoTHub, 0)
			senderRefreshTk.Stop()
			deviceStatusCheckTk.Stop()

			err := hub.Close(c.ctx)
			if err != nil {
				c.log.I("failed to close event hub", log.Error(err))
			}

			//closeSender()

			timer := time.NewTimer(0)
			if !timer.Stop() {
				<-timer.C
			}

			defer timer.Stop()

			b := backoff.NewStrategy(time.Second, time.Minute, 1.5, 0)

			for {
				select {
				case <-c.ctx.Done():
					// connectivity manager exited, do nothing
					return
				default:
					// connectivity manager alive, but somehow connection errored
					// we should reconnect to iot hub and event hub
					timer.Reset(b.Next("conn"))

					select {
					case <-timer.C:
						// backoff before reconnect
					case <-c.ctx.Done():
						return
					}

					err := c.Connect()
					if err != nil {
						c.log.I("failed to reconnect to iot hub", log.Error(err))
						continue
					}
					// connected, hand over to the new goroutine

					return
				}
			}
		}()

		for {
			select {
			case <-c.senderClosedSig:
				return
			case <-connCtx.Done():
				return
			case <-deviceStatusCheckTk.C:
				// check device connection since azure-iot-hub won't retain will message
				deviceConnected, err := c.checkDeviceConnected()
				if err != nil {
					c.log.I("failed to check device status", log.Error(err))
					return
				}

				if deviceConnected {
					c.onRecvMsg(c.onlineMsg)
				} else {
					c.onRecvMsg(c.offlineMsg)
				}
			case <-senderRefreshTk.C:
				sender, closeSender1, err := c.createSender()
				if err != nil {
					c.log.I("failed to refresh sender before token expired", log.Error(err))
					return
				}
				atomic.StoreUint32(&c.connectedToIoTHub, 0)

				closeSender()
				closeSender = closeSender1
				c.senderStore.Store(sender)

				atomic.StoreUint32(&c.connectedToIoTHub, 1)
			case <-c.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (c *azureIoTHubClient) Disconnect() error {
	c.exit()
	return nil
}

func (c *azureIoTHubClient) Subscribe() error {
	return nil
}

func (c *azureIoTHubClient) Publish(data []byte) error {
	msgID, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to create msgID: %w", err)
	}

	sender := c.getSender()
	if sender == nil {
		return fmt.Errorf("iot hub sender not available")
	}

	err = sender.Send(c.ctx, &amqp.Message{
		Data: [][]byte{data},
		ApplicationProperties: map[string]interface{}{
			"iothub-ack": "full",
		},
		Properties: &amqp.MessageProperties{
			MessageID: msgID.String(),
			To:        c.msgPropTo,
		},
	})

	if err != nil {
		if atomic.LoadUint32(&c.connectedToIoTHub) == 1 {
			select {
			case c.senderClosedSig <- struct{}{}:
				// signal to recreate sender iot hub
			case <-c.ctx.Done():
				// do nothing since connectivity manager has exited
			}
		}

		return err
	}

	return nil
}

func (c *azureIoTHubClient) handleEvent(ctx context.Context, event *eventhub.Event) error {
	if event.Properties == nil {
		return fmt.Errorf("unexpected empty properties")
	}

	if _, ok := event.Properties["arhat"]; !ok {
		return fmt.Errorf("arhat identifier not found")
	}

	var (
		deviceID string
	)

	v, ok := event.Properties["dev"]
	if !ok {
		return fmt.Errorf("device id not found")
	}

	if deviceID, ok = v.(string); !ok {
		return fmt.Errorf("invalid client id")
	}

	if deviceID != c.deviceID {
		return fmt.Errorf("device id not match")
	}

	c.onRecvMsg(event.Data)

	return nil
}

func (c *azureIoTHubClient) checkDeviceConnected() (bool, error) {
	token, err := c.iotHubTokenProvider.GetToken(fmt.Sprintf("%s/devices/%s", c.iotHubParsedConn.Host, c.deviceID))
	if err != nil {
		return false, fmt.Errorf("failed to generate token for iot hub http request: %w", err)
	}

	reqURL := fmt.Sprintf("https://%s/twins/%s?api-version=2018-06-30", c.iotHubParsedConn.Host, c.deviceID)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", token.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send device twin request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get device twin status, code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read device twin resp: %w", err)
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(data, &m)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal device twin resp: %w", err)
	}

	if v, ok := m["connectionState"]; ok {
		if connState, ok := v.(string); ok {
			if connState == "Connected" {
				return true, nil
			}
		}
	}

	return false, nil
}

func parseIoTHubConnectionString(str string) (*conn.ParsedConn, error) {
	parsed := new(conn.ParsedConn)
	for _, kv := range strings.Split(str, ";") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key value pair in conn str")
		}

		value := parts[1]
		switch strings.ToLower(parts[0]) {
		case "hostname":
			parsed.Host = value
			hostParts := strings.SplitN(value, ".", 2)
			if len(hostParts) != 2 {
				return nil, fmt.Errorf("invalid HostName entry: %s", value)
			}
			parsed.HubName = hostParts[0]
		case "sharedaccesskeyname":
			parsed.KeyName = value
		case "sharedaccesskey":
			data, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return nil, fmt.Errorf("failed to decode access key: %w", err)
			}
			parsed.Key = string(data)
		}
	}

	switch "" {
	case parsed.KeyName, parsed.Key, parsed.Host, parsed.HubName:
		return nil, fmt.Errorf("invalid connection string")
	}

	return parsed, nil
}
