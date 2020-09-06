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
	"os"

	"github.com/spf13/cobra"
)

type versionOptions struct {
	branch             bool
	commit             bool
	tag                bool
	arch               bool
	goVersion          bool
	goCompilerPlatform bool
}

func NewVersionCmd() *cobra.Command {
	opt := new(versionOptions)
	versionCmd := &cobra.Command{
		Use:          "version",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			show := func(s string) {
				_, _ = fmt.Fprint(os.Stdout, s)
			}

			switch {
			case opt.branch:
				show(Branch())
			case opt.commit:
				show(Commit())
			case opt.tag:
				show(Tag())
			case opt.arch:
				show(Arch())
			case opt.goVersion:
				show(GoVersion())
			case opt.goCompilerPlatform:
				show(GoCompilerPlatform())
			default:
				show(Version())
			}
		},
	}

	versionFlags := versionCmd.Flags()
	versionFlags.BoolVar(&opt.branch, "branch", false, "get branch name")
	versionFlags.BoolVar(&opt.commit, "commit", false, "get commit hash")
	versionFlags.BoolVar(&opt.tag, "tag", false, "get tag name")
	versionFlags.BoolVar(&opt.arch, "arch", false, "get arch")
	versionFlags.BoolVar(&opt.goVersion, "go.version", false, "get go runtime/compiler version")
	versionFlags.BoolVar(&opt.goCompilerPlatform, "go.compilerPlatform", false, "get os/arch pair of the go compiler")

	return versionCmd
}
