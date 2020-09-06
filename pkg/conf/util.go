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

package conf

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"arhat.dev/template-kubernetes-controller/pkg/constant"
)

type Config interface {
	GetLogConfig() log.ConfigSet
	SetLogConfig(log.ConfigSet)
}

func ReadConfig(
	cmd *cobra.Command,
	configFile *string,
	cliLogConfig *log.Config,
	config Config,
) (context.Context, error) {
	flags := cmd.Flags()
	configBytes, err := ioutil.ReadFile(*configFile)
	if err != nil && flags.Changed("config") {
		return nil, fmt.Errorf("failed to read config file %s: %v", *configFile, err)
	}

	if len(configBytes) > 0 {
		configStr := envhelper.Expand(string(configBytes), func(s, origin string) string {
			// nolint:gocritic
			switch s {
			// TODO: add special cases if any
			default:
				v, found := os.LookupEnv(s)
				if found {
					return v
				}
				return origin
			}
		})

		if err = yaml.Unmarshal([]byte(configStr), config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config file %s: %v", *configFile, err)
		}
	}

	logConfigSet := config.GetLogConfig()
	if len(logConfigSet) > 0 {
		if flags.Changed("log.format") {
			logConfigSet[0].Format = cliLogConfig.Format
		}

		if flags.Changed("log.level") {
			logConfigSet[0].Level = cliLogConfig.Level
		}

		if flags.Changed("log.file") {
			logConfigSet[0].File = cliLogConfig.File
		}
	} else {
		logConfigSet = append(logConfigSet, *cliLogConfig)
	}
	config.SetLogConfig(logConfigSet)

	if err = cmd.ParseFlags(os.Args); err != nil {
		return nil, err
	}

	err = log.SetDefaultLogger(logConfigSet)
	if err != nil {
		return nil, fmt.Errorf("failed to set default logger: %w", err)
	}

	appCtx, exit := context.WithCancel(context.WithValue(context.Background(), constant.ContextKeyConfig, config))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		exitCount := 0
		for sig := range sigCh {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				exitCount++
				if exitCount == 1 {
					exit()
				} else {
					os.Exit(1)
				}
				//case syscall.SIGHUP:
				//	// force reload
			}
		}
	}()

	return appCtx, nil
}
