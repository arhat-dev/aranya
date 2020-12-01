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
	"errors"
	"fmt"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
)

var (
	ErrUnsupportedManager = errors.New("unsupported manager")
)

func NewMessageQueueManager(parentCtx context.Context, name string, mgrConfig *Options) (*MessageQueueManager, error) {
	var (
		err error
		mgr = &MessageQueueManager{baseManager: nil}
	)

	mgr.baseManager, err = newBaseManager(parentCtx, name, mgrConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base manager: %w", err)
	}

	switch {
	case mgrConfig.MQTTOpts != nil:
		mgr.client, err = newMQTTClient(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create mqtt client: %w", err)
		}
	case mgrConfig.AMQPOpts != nil:
		mgr.client, err = newAMQPClient(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create amqp client: %w", err)
		}
	case mgrConfig.AzureIoTHubOpts != nil:
		mgr.client, err = newAzureIoTHubClient(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create iot hub client: %w", err)
		}
	case mgrConfig.GCPIoTCoreOpts != nil:
		mgr.client, err = newGCPIoTCoreClient(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create gcp pubsub client: %w", err)
		}
	default:
		return nil, ErrUnsupportedManager
	}

	err = mgr.client.Connect()
	if err != nil {
		_ = mgr.client.Disconnect()
		return nil, fmt.Errorf("failed to connect to broker: %v", err)
	}

	defer func() {
		if err != nil {
			mgr.log.I("disconnect from broker due to error")
			_ = mgr.client.Disconnect()
		}
	}()

	err = mgr.client.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe topic %v: %v", mgr.client.SubTopics(), err)
	}

	mgr.sendCmd = func(cmd *aranyagopb.Cmd) error {
		data, err := cmd.Marshal()
		if err != nil {
			return err
		}

		return mgr.client.Publish(data)
	}

	mgr.log.I("created mq client",
		log.Strings("subTopics", mgr.client.SubTopics()),
		log.Strings("pubTopics", mgr.client.PubTopics()),
	)

	return mgr, nil
}

type MessageQueueManager struct {
	*baseManager

	client messageQueueClient
}

func (m *MessageQueueManager) Start() error {
	// nolint:gosimple
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
}

func (m *MessageQueueManager) Close() {
	m.OnDisconnected(func() (id string, all bool) {
		_ = m.client.Disconnect()
		return "", true
	})

	m.onClose(func() {})
}

func (m *MessageQueueManager) Reject(reason aranyagopb.RejectionReason, message string) {
	m.onReject(func() {
		// best effort
		if m.sendCmd != nil {
			data, _ := (&aranyagopb.RejectCmd{
				Reason:  reason,
				Message: message,
			}).Marshal()

			_ = m.sendCmd(&aranyagopb.Cmd{
				Kind:      aranyagopb.CMD_REJECT,
				Sid:       0,
				Seq:       0,
				Completed: true,
				Payload:   data,
			})
		}
	})
}

type messageQueueClient interface {
	Connect() error
	Disconnect() error
	Subscribe() error
	Publish(data []byte) error

	PubTopics() []string
	SubTopics() []string
}
