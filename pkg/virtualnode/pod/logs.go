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

package pod

import (
	"fmt"
	"path/filepath"
)

func podLogDir(logRootDir, namespace, podName, podUID string) string {
	return filepath.Join(logRootDir, fmt.Sprintf("%s_%s_%s", namespace, podName, podUID))
}

func containerLogFile(podLogDir, containerName string) string {
	return filepath.Join(podLogDir, containerName, "1.log")
}
