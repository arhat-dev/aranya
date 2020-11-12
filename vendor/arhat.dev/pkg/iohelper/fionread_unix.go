// +build !windows,!plan9,!solaris,!aix,!js

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
	"syscall"
	"unsafe"
)

// CheckBytesToRead calls ioctl(fd, FIONREAD) to check ready data size of fd
func CheckBytesToRead(fd uintptr) (int, error) {
	var value int
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, _FIONREAD, uintptr(unsafe.Pointer(&value)))
	if errno != 0 {
		return 0, errno
	}

	return value, nil
}
