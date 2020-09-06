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

package version

import (
	"fmt"
	"runtime"
)

var (
	branch, commit, tag, arch string
	goCompilerPlatform        string
)

var version string

func init() {
	version = fmt.Sprintf(`branch: %s
commit: %s
tag: %s
arch: %s
goVersion: %s
goCompilerPlatform: %s
`, Branch(), Commit(), Tag(), Arch(), GoVersion(), GoCompilerPlatform())
}

func Version() string {
	return version
}

// Branch name of the source code
func Branch() string {
	return branch
}

// Commit hash of the source code
func Commit() string {
	return commit
}

// Tag the tag name of the source code
func Tag() string {
	return tag
}

func Arch() string {
	return arch
}

func GoVersion() string {
	return runtime.Version()
}

func GoCompilerPlatform() string {
	return goCompilerPlatform
}
