package edgedevice

import (
	"errors"
	"fmt"

	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
)

// check existing abbot endpoints, EdgeDevices' network config (enabled or not)
func (c *Controller) onNetworkEnsureRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkDeleteRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) requestNetworkEnsure(name string) error {
	if c.netReqRec == nil {
		return fmt.Errorf("network ensure not supported")
	}

	c.netReqRec.Update(name, name, name)
	err := c.netReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule network ensure: %w", err)
	}

	return nil
}
