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

package textquery

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/itchyny/gojq"

	"arhat.dev/pkg/decodehelper"
)

func JQ(query, data string) (string, error) {
	return JQBytes(query, []byte(data))
}

func JQBytes(query string, input []byte) (string, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return "", fmt.Errorf("failed to parse query: %w", err)
	}

	strData, err := strconv.Unquote(string(input))
	if err == nil {
		input = []byte(strData)
	}

	var data interface{}

	mapData := make(map[string]interface{})
	err = decodehelper.UnmarshalJSON(input, &mapData)
	data = mapData

	if err != nil {
		// maybe it's an array
		var arrayData []interface{}
		err = decodehelper.UnmarshalJSON(input, &arrayData)
		data = arrayData
	}

	if err != nil {
		// maybe it's anything else
		var anyData interface{}
		err = decodehelper.UnmarshalJSON(input, &anyData)
		data = anyData
	}

	if err != nil {
		// maybe it's plain text
		data = input
	}

	result, _, err := RunQuery(q, data, nil)
	return result, err
}

func RunQuery(query *gojq.Query, data interface{}, kvPairs map[string]interface{}) (string, bool, error) {
	var iter gojq.Iter

	if len(kvPairs) == 0 {
		iter = query.Run(data)
	} else {
		var (
			keys   []string
			values []interface{}
		)
		for k, v := range kvPairs {
			keys = append(keys, k)
			values = append(values, v)
		}

		code, err := gojq.Compile(query, gojq.WithVariables(keys))
		if err != nil {
			return "", false, fmt.Errorf("failed to compile query with variables: %w", err)
		}

		iter = code.Run(data, values...)
	}

	var result []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			return "", false, err
		}

		result = append(result, v)
	}

	switch len(result) {
	case 0:
		return "", false, nil
	case 1:
		switch r := result[0].(type) {
		case string:
			return r, true, nil
		case []byte:
			return string(r), true, nil
		case []interface{}, map[string]interface{}:
			res, err := json.Marshal(r)
			return string(res), true, err
		case int64:
			return strconv.FormatInt(r, 10), true, nil
		case int32:
			return strconv.FormatInt(int64(r), 10), true, nil
		case int16:
			return strconv.FormatInt(int64(r), 10), true, nil
		case int8:
			return strconv.FormatInt(int64(r), 10), true, nil
		case int:
			return strconv.FormatInt(int64(r), 10), true, nil
		case uint64:
			return strconv.FormatUint(r, 10), true, nil
		case uint32:
			return strconv.FormatUint(uint64(r), 10), true, nil
		case uint16:
			return strconv.FormatUint(uint64(r), 10), true, nil
		case uint8:
			return strconv.FormatUint(uint64(r), 10), true, nil
		case uint:
			return strconv.FormatUint(uint64(r), 10), true, nil
		case float64:
			return strconv.FormatFloat(r, 'f', -1, 64), true, nil
		case float32:
			return strconv.FormatFloat(float64(r), 'f', -1, 64), true, nil
		case bool:
			if r {
				return "true", true, nil
			}
			return "false", true, nil
		case nil:
			return "null", true, nil
		default:
			return fmt.Sprintf("%v", r), true, nil
		}
	default:
		res, err := json.Marshal(result)
		return string(res), true, err
	}
}
