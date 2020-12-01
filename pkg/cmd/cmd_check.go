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

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"unsafe"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type checkOpts struct {
	log bool
}

const checkCount = unsafe.Sizeof(checkOpts{}) / unsafe.Sizeof(true)

func newCheckCmd(appCtx *context.Context) *cobra.Command {
	var (
		opts     = new(checkOpts)
		checkAll bool
	)

	checkCmd := &cobra.Command{
		Use:           "check",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// nolint:gocritic
			switch {
			case opts.log:
				checkAll = false
			}

			if checkAll {
				opts.log = true
			}

			return runCheck(*appCtx, (*appCtx).Value(constant.ContextKeyConfig).(*conf.Config), opts)
		},
	}

	flags := checkCmd.Flags()

	flags.BoolVar(&checkAll, "all", true, "check all")
	flags.BoolVar(&opts.log, "log", false, "check log config")

	return checkCmd
}

func runCheck(appCtx context.Context, config *conf.Config, opts *checkOpts) error {
	_ = appCtx

	showResult := func(name string, val interface{}) {
		if err, ok := val.(error); ok {
			fmt.Printf("%s: err(%v)\n", name, err)
		} else {
			fmt.Printf("%s: %v\n", name, val)
		}
	}

	configBytes, err := yaml.Marshal(config)
	if err != nil {
		showResult("config", err)
	} else {
		fmt.Println("config: |")
		s := bufio.NewScanner(bytes.NewReader(configBytes))
		s.Split(bufio.ScanLines)
		for s.Scan() {
			fmt.Printf("  %s\n", s.Text())
		}
	}

	fmt.Println("---")

	for i := checkCount; i > 0; i-- {
		// nolint:gocritic
		switch {
		case opts.log:
			opts.log = false
			for i, logConfig := range config.Aranya.Log.GetUnique() {
				showResult(fmt.Sprintf("log[%d]", i),
					fmt.Sprintf("%s@%s:%s", logConfig.Destination.File, logConfig.Format, logConfig.Level))
			}
		}
	}

	return nil
}
