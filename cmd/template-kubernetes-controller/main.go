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

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"arhat.dev/template-kubernetes-controller/cmd/template-kubernetes-controller/pkg"
	"arhat.dev/template-kubernetes-controller/pkg/version"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	cmd := pkg.NewTemplateKubernetesControllerCmd()
	cmd.AddCommand(version.NewVersionCmd())

	err := cmd.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to run template-kubernetes-controller %v: %v\n", os.Args, err)
		os.Exit(1)
	}
}
