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

package virtualnode

import (
	"fmt"
	"math/big"
	"runtime"
	"sync/atomic"

	"arhat.dev/aranya-proto/aranyagopb"
	corev1 "k8s.io/api/core/v1"
)

func newNodeCache(nodeLabels, nodeAnnotations map[string]string) *NodeCache {
	if nodeLabels == nil {
		nodeLabels = make(map[string]string)
	}

	if nodeAnnotations == nil {
		nodeAnnotations = make(map[string]string)
	}

	return &NodeCache{
		nodeLabels:      nodeLabels,
		nodeAnnotations: nodeAnnotations,

		status: new(corev1.NodeStatus),
	}
}

// NodeCache is a thread-safe cache store for node status and ext info (extra labels and annotations)
type NodeCache struct {
	status *corev1.NodeStatus

	nodeLabels, nodeAnnotations map[string]string

	extLabels, extAnnotations map[string]string

	_working uint32
}

func (n *NodeCache) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&n._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&n._working, 0)
}

func (n *NodeCache) UpdatePhase(p corev1.NodePhase) {
	if p == "" {
		return
	}

	n.doExclusive(func() { n.status.Phase = p })
}

func (n *NodeCache) UpdateCapacity(c corev1.ResourceList) {
	if len(c) == 0 {
		return
	}

	n.doExclusive(func() { n.status.Capacity = c })
}

func (n *NodeCache) UpdateConditions(c []corev1.NodeCondition) {
	if len(c) == 0 {
		return
	}

	n.doExclusive(func() { n.status.Conditions = c })
}

func (n *NodeCache) UpdateSystemInfo(i *corev1.NodeSystemInfo) {
	if i == nil {
		return
	}

	n.doExclusive(func() { n.status.NodeInfo = *i })
}

func (n *NodeCache) UpdateExtInfo(extInfo []*aranyagopb.NodeExtInfo) error {
	// apply MUST be atomic
	apply := func() error {
		extLabels := make(map[string]string)
		extAnnotations := make(map[string]string)

		for _, info := range extInfo {
			var (
				target, oldExtValues, nodeValues *map[string]string
			)
			switch info.Target {
			case aranyagopb.NODE_EXT_INFO_TARGET_ANNOTATION:
				target, oldExtValues, nodeValues = &extAnnotations, &n.extAnnotations, &n.nodeAnnotations
			case aranyagopb.NODE_EXT_INFO_TARGET_LABEL:
				target, oldExtValues, nodeValues = &extLabels, &n.extLabels, &n.nodeLabels
			default:
				return fmt.Errorf("invalid ext info target %q", info.Target.String())
			}

			oldVal, ok := (*oldExtValues)[info.TargetKey]
			if !ok {
				oldVal = (*nodeValues)[info.TargetKey]
			}

			switch info.ValueType {
			case aranyagopb.NODE_EXT_INFO_TYPE_STRING:
				switch info.Operator {
				case aranyagopb.NODE_EXT_INFO_OPERATOR_SET:
					(*target)[info.TargetKey] = info.Value
				case aranyagopb.NODE_EXT_INFO_OPERATOR_ADD:
					(*target)[info.TargetKey] = oldVal + info.Value
				default:
					return fmt.Errorf("invalid operator for string ext info %q", info.Operator.String())
				}
			case aranyagopb.NODE_EXT_INFO_TYPE_NUMBER:
				resolvedVal, err := n.calculateNumber(oldVal, info.Value, info.Operator)
				if err != nil {
					return fmt.Errorf("failed to calculate number of : %w", err)
				}
				(*target)[info.TargetKey] = resolvedVal
			default:
				return fmt.Errorf("unsupported ext info value type %q", info.ValueType.String())
			}
		}

		n.extLabels = extLabels
		n.extAnnotations = extAnnotations

		return nil
	}

	var err error
	n.doExclusive(func() {
		err = apply()
	})

	return err
}

func (n *NodeCache) RetrieveExtInfo() (extLabels, extAnnotations map[string]string) {
	extLabels, extAnnotations = make(map[string]string), make(map[string]string)

	n.doExclusive(func() {
		for k, v := range n.extLabels {
			extLabels[k] = v
		}

		for k, v := range n.extAnnotations {
			extAnnotations[k] = v
		}
	})

	return
}

// Retrieve latest node status cache
func (n *NodeCache) RetrieveStatus(s corev1.NodeStatus) corev1.NodeStatus {
	var status *corev1.NodeStatus
	n.doExclusive(func() { status = n.status.DeepCopy() })

	result := s.DeepCopy()
	result.Phase = status.Phase
	result.Capacity = status.Capacity
	result.Conditions = status.Conditions
	result.NodeInfo = status.NodeInfo

	return *result
}

func (n *NodeCache) calculateNumber(oldVal, v string, op aranyagopb.NodeExtInfo_Operator) (string, error) {
	newNum, _, err := new(big.Float).Parse(v, 0)
	if err != nil {
		return "", err
	}

	var oldNum *big.Float
	if len(oldVal) == 0 {
		oldNum = big.NewFloat(0)
	} else {
		oldNum, _, err = new(big.Float).Parse(oldVal, 0)
		if err != nil {
			return "", err
		}
	}

	switch op {
	case aranyagopb.NODE_EXT_INFO_OPERATOR_SET:
		return newNum.Text('f', -1), nil
	case aranyagopb.NODE_EXT_INFO_OPERATOR_ADD:
		return oldNum.Add(oldNum, newNum).Text('f', -1), nil
	case aranyagopb.NODE_EXT_INFO_OPERATOR_MINUS:
		return oldNum.Add(oldNum, newNum.Neg(newNum)).Text('f', -1), nil
	default:
		return "", fmt.Errorf("unsupported operator for number %q", op.String())
	}
}
