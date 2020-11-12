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

package runtimepb

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
)

func (s *ContainerStatus) GetState() PodState {
	if s == nil {
		return POD_STATE_UNKNOWN
	}

	createdAt, startedAt, finishedAt := s.GetTimeCreatedAt(), s.GetTimeStartedAt(), s.GetTimeFinishedAt()

	if !startedAt.After(finishedAt) {
		if s.ExitCode != 0 {
			return POD_STATE_FAILED
		}

		return POD_STATE_SUCCEEDED
	}

	if !startedAt.IsZero() {
		return POD_STATE_RUNNING
	}

	if !createdAt.IsZero() {
		return POD_STATE_PENDING
	}

	if s.ContainerId == "" {
		return POD_STATE_UNKNOWN
	}

	return POD_STATE_PENDING
}

func (v *ContainerMountSpec) Ensure(dir string, dataMap map[string][]byte) (mountPath string, err error) {
	fileMode := os.FileMode(0644)

	if m := v.FileMode; m > 0 && m <= 0777 {
		fileMode = os.FileMode(m)
	}

	if subPath := v.GetSubPath(); subPath != "" {
		data, ok := dataMap[subPath]
		if !ok {
			return "", errors.New("volume data not found")
		}

		dataFilePath := filepath.Join(dir, subPath)
		if err = ioutil.WriteFile(dataFilePath, data, fileMode); err != nil {
			return "", err
		}
		return dataFilePath, nil
	}

	for fileName, data := range dataMap {
		dataFilePath := filepath.Join(dir, fileName)
		if err = ioutil.WriteFile(dataFilePath, data, fileMode); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func (s *ContainerStatus) GetTimeCreatedAt() time.Time {
	t, _ := time.Parse(aranyagoconst.TimeLayout, s.CreatedAt)
	return t
}

func (s *ContainerStatus) GetTimeStartedAt() time.Time {
	t, _ := time.Parse(aranyagoconst.TimeLayout, s.StartedAt)
	return t
}

func (s *ContainerStatus) GetTimeFinishedAt() time.Time {
	t, _ := time.Parse(aranyagoconst.TimeLayout, s.FinishedAt)
	return t
}
