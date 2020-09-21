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
	"fmt"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	"cloud.google.com/go/pubsub"
	"google.golang.org/api/cloudiot/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newGCPIoTCoreClient(parent *MessageQueueManager) (*gcpIoTCoreClient, error) {
	opts := parent.config.GCPIoTCoreOpts

	ctx, exit := context.WithCancel(parent.ctx)

	psClient, err := pubsub.NewClient(
		ctx,
		opts.Config.ProjectID,
		option.WithCredentialsJSON(opts.PubSubCredentialsJSON),
	)
	if err != nil {
		exit()
		return nil, fmt.Errorf("failed to create gcp pubsub api client: %w", err)
	}

	iotClient, err := cloudiot.NewService(ctx, option.WithCredentialsJSON(opts.CloudIoTCredentialsJSON))
	if err != nil {
		exit()
		return nil, fmt.Errorf("failed to create gcp cloud iot api client: %w", err)
	}

	stateTopicStr := opts.Config.PubSub.StateTopicID
	if stateTopicStr == "" {
		stateTopicStr = opts.Config.PubSub.TelemetryTopicID
	}

	pollInterval := opts.Config.CloudIoT.DeviceStatusPollInterval.Duration
	if pollInterval == 0 {
		pollInterval = time.Minute
	}

	onlineMsgBytes, _ := aranyagopb.NewOnlineStateMsg(opts.Config.CloudIoT.DeviceID).Marshal()
	offlineMsgBytes, _ := aranyagopb.NewOfflineStateMsg(opts.Config.CloudIoT.DeviceID).Marshal()

	client := &gcpIoTCoreClient{
		log:          parent.log,
		ctx:          ctx,
		exit:         exit,
		deviceID:     opts.Config.CloudIoT.DeviceID,
		pollInterval: pollInterval,

		psClient:   psClient,
		iotClient:  iotClient,
		msgTopic:   psClient.Topic(opts.Config.PubSub.TelemetryTopicID),
		stateTopic: psClient.Topic(stateTopicStr),
		cmdTopic: fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s",
			opts.Config.ProjectID, opts.Config.CloudIoT.Region,
			opts.Config.CloudIoT.RegistryID, opts.Config.CloudIoT.DeviceID),

		onRecvMsg: parent.onRecvMsg,
		rejectAgent: func() {
			parent.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "gcp iot core connection lost")
		},
		onlineMsg:  aranyagopb.NewMsg(aranyagopb.MSG_STATE, 0, 0, true, onlineMsgBytes),
		offlineMsg: aranyagopb.NewMsg(aranyagopb.MSG_STATE, 0, 0, true, offlineMsgBytes),
	}

	return client, nil
}

type gcpIoTCoreClient struct {
	log          log.Interface
	ctx          context.Context
	exit         context.CancelFunc
	deviceID     string
	pollInterval time.Duration

	psClient   *pubsub.Client
	iotClient  *cloudiot.Service
	msgTopic   *pubsub.Topic
	stateTopic *pubsub.Topic
	cmdTopic   string

	onRecvMsg   func(*aranyagopb.Msg)
	rejectAgent func()
	onlineMsg   *aranyagopb.Msg
	offlineMsg  *aranyagopb.Msg
}

func (c *gcpIoTCoreClient) Connect() error {
	_, err := c.checkDeviceStatus()
	if err != nil {
		return fmt.Errorf("failed to check device status: %w", err)
	}

	return nil
}

func (c *gcpIoTCoreClient) Disconnect() error {
	c.stateTopic.Stop()
	c.msgTopic.Stop()
	c.exit()
	return nil
}

func (c *gcpIoTCoreClient) ensureSubscription(subID string, topic *pubsub.Topic) (*pubsub.Subscription, error) {
	createSub := false
	sub := c.psClient.Subscription(subID)
	config, err := sub.Config(c.ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, fmt.Errorf("failed to check config of subscription %s: %w", subID, err)
		}
		createSub = true
	}

	if config.Topic != nil {
		if config.Topic.String() != topic.String() {
			createSub = true
			err2 := sub.Delete(c.ctx)
			if err2 != nil {
				return nil, fmt.Errorf("failed to delete bad subscription %s: %w", subID, err2)
			}
		}
	} else {
		createSub = true
	}

	if createSub {
		sub, err = c.psClient.CreateSubscription(c.ctx, subID, pubsub.SubscriptionConfig{
			Topic:               topic,
			RetainAckedMessages: false,
			AckDeadline:         10 * time.Second,
			RetentionDuration:   10 * time.Minute,
			ExpirationPolicy:    24 * time.Hour,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription %s: %w", subID, err)
		}
	}

	return sub, nil
}

func (c *gcpIoTCoreClient) Subscribe() error {
	msgSub, err := c.ensureSubscription(fmt.Sprintf("aranya.%s.msg", c.deviceID), c.msgTopic)
	if err != nil {
		return fmt.Errorf("failed to ensure msg sub: %w", err)
	}

	stateSub, err := c.ensureSubscription(fmt.Sprintf("aranya.%s.state", c.deviceID), c.stateTopic)
	if err != nil {
		return fmt.Errorf("failed to ensure state sub: %w", err)
	}

	go func() {
		tk := time.NewTicker(c.pollInterval)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				online, err := c.checkDeviceStatus()
				if err != nil {
					c.log.I("failed to check device status", log.Error(err))
					break
				}

				var msg *aranyagopb.Msg
				if online {
					msg = c.onlineMsg
				} else {
					msg = c.offlineMsg
				}

				data, _ := msg.Marshal()

				c.handleRecv(c.ctx, &pubsub.Message{
					Attributes: map[string]string{
						"deviceId": c.deviceID,
					},
					Data: data,
				})
			case <-c.ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := msgSub.Receive(c.ctx, c.handleRecv); err != nil {
					c.rejectAgent()
					c.log.I("failed to create receive from gcp pubsub for device msg", log.Error(err))
				}
			}
		}
	}()

	if c.stateTopic.String() != c.msgTopic.String() {
		go func() {
			for {
				select {
				case <-c.ctx.Done():
					return
				default:
					if err := stateSub.Receive(c.ctx, c.handleRecv); err != nil {
						c.log.I("failed to create receive from gcp pubsub for device state", log.Error(err))
					}
				}
			}
		}()
	}

	return nil
}

func (c *gcpIoTCoreClient) Publish(data []byte) error {
	resp, err := c.iotClient.Projects.Locations.Registries.Devices.
		SendCommandToDevice(c.cmdTopic, &cloudiot.SendCommandToDeviceRequest{
			BinaryData: base64.StdEncoding.EncodeToString(data),
		}).
		Context(c.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to request send iot core command: %w", err)
	}
	_ = resp

	return nil
}

func (c *gcpIoTCoreClient) PubTopics() []string {
	return []string{c.cmdTopic}
}

func (c *gcpIoTCoreClient) SubTopics() []string {
	return []string{c.msgTopic.String(), c.stateTopic.String()}
}

func (c *gcpIoTCoreClient) checkDeviceStatus() (online bool, _ error) {
	defer func() {
		if online {
			c.onRecvMsg(c.onlineMsg)
		} else {
			c.onRecvMsg(c.offlineMsg)
		}
	}()

	resp, err := c.iotClient.Projects.Locations.Registries.Devices.Get(c.cmdTopic).Context(c.ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to request device info: %w", err)
	}

	if resp.State == nil {
		return false, nil
	}

	errTime, err := time.Parse(time.RFC3339Nano, resp.LastErrorTime)
	if err != nil {
		return false, fmt.Errorf("bad error time %s: %w", resp.LastErrorTime, err)
	}

	onlineTime, err := time.Parse(time.RFC3339Nano, resp.State.UpdateTime)
	if err != nil {
		return false, fmt.Errorf("bad state update time %s: %w", resp.LastErrorTime, err)
	}

	if onlineTime.After(errTime) {
		return true, nil
	}

	return false, nil
}

func (c *gcpIoTCoreClient) handleRecv(ctx context.Context, msg *pubsub.Message) {
	if len(msg.Attributes) == 0 || msg.Attributes["deviceId"] != c.deviceID {
		// unwanted message
		msg.Nack()
		return
	}

	m := new(aranyagopb.Msg)
	err := m.Unmarshal(msg.Data)
	if err != nil {
		msg.Nack()
		c.log.I("failed to unmarshal received data", log.Error(err))
		return
	}

	msg.Ack()

	go c.onRecvMsg(m)
}
