package patchhelper

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func TwoWayMergePatch(oldOne, newOne, kind interface{}, doPatch func(patchData []byte) error) error {
	oldOneData, err := json.Marshal(oldOne)
	if err != nil {
		return fmt.Errorf("failed to marshal old one: %w", err)
	}

	newOneData, err := json.Marshal(newOne)
	if err != nil {
		return fmt.Errorf("failed to marshal new one: %w", err)
	}

	patchData, err := strategicpatch.CreateTwoWayMergePatch(oldOneData, newOneData, kind)
	if err != nil {
		return fmt.Errorf("failed to generate patch data: %w", err)
	}

	return doPatch(patchData)
}
