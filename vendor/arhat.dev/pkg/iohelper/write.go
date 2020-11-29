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

package iohelper

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// WriteFile write data to target file
func WriteFile(file string, data []byte, perm os.FileMode, overwrite bool) (undo func() error, _ error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("failed to determine absolute path of file: %w", err)
	}

	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = filepath.VolumeName(file)
	}

	f, err := os.Stat(file)
	if err == nil {
		if f.IsDir() {
			return nil, fmt.Errorf("target file is a directory")
		}

		if !overwrite {
			return func() error { return nil }, nil
		}

		// need to overwrite old file content

		var oldData []byte
		oldData, err = ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read old file content for undo: %w", err)
		}

		err = ioutil.WriteFile(file, data, perm)
		if err != nil {
			return func() error { return nil }, fmt.Errorf("failed to overwrite old file content: %w", err)
		}

		return func() error {
			undoErr := ioutil.WriteFile(file, oldData, f.Mode()&os.ModePerm)
			if undoErr != nil {
				return fmt.Errorf("failed to restore old file content: %w", undoErr)
			}

			return nil
		}, nil
	}

	// file not exists, need to create

	dir := filepath.Dir(file)
	dirParts := strings.Split(dir, fmt.Sprintf("%c", os.PathSeparator))

	missingDirIndex := -1
	for i := len(dirParts) - 1; i >= 0; i-- {
		thisDir := filepath.Join(rootPath, filepath.Join(dirParts[:i]...))
		_, err = os.Stat(thisDir)
		if err != nil {
			continue
		}

		// if the directory to create file exists, no need to create dir
		if i == len(dirParts)-1 {
			break
		}

		// need to create all subsequent dirs in this dir
		missingDirIndex = i + 1
	}

	var undoActions []func() error

	if missingDirIndex != -1 {
		createdDir := filepath.Join(rootPath, filepath.Join(dirParts[:missingDirIndex+1]...))
		undoActions = append(undoActions, func() error {
			return os.Remove(createdDir)
		})

		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory for file: %w", err)
		}
	}

	buildUndoFunc := func() func() error {
		return func() error {
			var undoErr error
			for i := len(undoActions) - 1; i >= 0; i-- {
				err2 := undoActions[i]()
				if err2 != nil {
					if undoErr != nil {
						undoErr = fmt.Errorf("%s; %w", undoErr.Error(), err2)
					} else {
						undoErr = err2
					}
				}
			}

			return undoErr
		}
	}

	err = ioutil.WriteFile(file, data, perm)
	if err != nil {
		return buildUndoFunc(), fmt.Errorf("failed to write file content: %w", err)
	}

	undoActions = append(undoActions, func() error {
		return os.Remove(file)
	})

	return buildUndoFunc(), nil
}
