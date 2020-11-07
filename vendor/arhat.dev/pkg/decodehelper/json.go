/*
Copyright 2015 The Kubernetes Authors.

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

package decodehelper

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// limit recursive depth to prevent stack overflow errors
const maxDepth = 10000

// UnmarshalJSON unmarshals the given data
// If v is a *map[string]interface{}, numbers are converted to int64 or float64
func UnmarshalJSON(data []byte, v interface{}) error {
	switch v := v.(type) {
	case *map[string]interface{}:
		// Build a decoder from the given data
		decoder := json.NewDecoder(bytes.NewBuffer(data))
		// Preserve numbers, rather than casting to float64 automatically
		decoder.UseNumber()
		// Run the decode
		if err := decoder.Decode(v); err != nil {
			return err
		}
		// If the decode succeeds, post-process the map to convert json.Number objects to int64 or float64
		return convertMapNumbers(*v, 0)

	case *[]interface{}:
		// Build a decoder from the given data
		decoder := json.NewDecoder(bytes.NewBuffer(data))
		// Preserve numbers, rather than casting to float64 automatically
		decoder.UseNumber()
		// Run the decode
		if err := decoder.Decode(v); err != nil {
			return err
		}
		// If the decode succeeds, post-process the map to convert json.Number objects to int64 or float64
		return convertSliceNumbers(*v, 0)

	case *interface{}:
		// Build a decoder from the given data
		decoder := json.NewDecoder(bytes.NewBuffer(data))
		// Preserve numbers, rather than casting to float64 automatically
		decoder.UseNumber()
		// Run the decode
		if err := decoder.Decode(v); err != nil {
			return err
		}
		// If the decode succeeds, post-process the map to convert json.Number objects to int64 or float64
		return convertInterfaceNumbers(v, 0)

	default:
		return json.Unmarshal(data, v)
	}
}

func convertInterfaceNumbers(v *interface{}, depth int) error {
	var err error
	switch v2 := (*v).(type) {
	case json.Number:
		*v, err = convertNumber(v2)
	case map[string]interface{}:
		err = convertMapNumbers(v2, depth+1)
	case []interface{}:
		err = convertSliceNumbers(v2, depth+1)
	}
	return err
}

// convertMapNumbers traverses the map, converting any json.Number values to int64 or float64.
// values which are map[string]interface{} or []interface{} are recursively visited
func convertMapNumbers(m map[string]interface{}, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("exceeded max depth of %d", maxDepth)
	}

	var err error
	for k, v := range m {
		switch v := v.(type) {
		case json.Number:
			m[k], err = convertNumber(v)
		case map[string]interface{}:
			err = convertMapNumbers(v, depth+1)
		case []interface{}:
			err = convertSliceNumbers(v, depth+1)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// convertSliceNumbers traverses the slice, converting any json.Number values to int64 or float64.
// values which are map[string]interface{} or []interface{} are recursively visited
func convertSliceNumbers(s []interface{}, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("exceeded max depth of %d", maxDepth)
	}

	var err error
	for i, v := range s {
		switch v := v.(type) {
		case json.Number:
			s[i], err = convertNumber(v)
		case map[string]interface{}:
			err = convertMapNumbers(v, depth+1)
		case []interface{}:
			err = convertSliceNumbers(v, depth+1)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// convertNumber converts a json.Number to an int64 or float64, or returns an error
func convertNumber(n json.Number) (interface{}, error) {
	// Attempt to convert to an int64 first
	if i, err := n.Int64(); err == nil {
		return i, nil
	}
	// Return a float64 (default json.Decode() behavior)
	// An overflow will return an error
	return n.Float64()
}
