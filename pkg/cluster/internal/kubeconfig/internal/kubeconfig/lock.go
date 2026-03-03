/*
Copyright 2019 The Kubernetes Authors.

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

package kubeconfig

import (
	"os"
	"path/filepath"
	"time"
)

// these are from
// https://github.com/kubernetes/client-go/blob/611184f7c43ae2d520727f01d49620c7ed33412d/tools/clientcmd/loader.go#L439-L440

func lockFile(filename string) error {
	lockPath := lockName(filename)
	// Make sure the dir exists before we try to create a lock file.
	dir := filepath.Dir(lockPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		// Check if lock is stale (older than 5 minutes)
		if os.IsExist(err) {
			if info, statErr := os.Stat(lockPath); statErr == nil {
				if time.Since(info.ModTime()) > 5*time.Minute {
					// Remove stale lock and retry once
					if rmErr := os.Remove(lockPath); rmErr != nil {
						return rmErr
					}
					f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0)
					if err != nil {
						return err
					}
					if err := f.Close(); err != nil {
						return err
					}
					return nil
				}
			}
		}
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func unlockFile(filename string) error {
	return os.Remove(lockName(filename))
}

func lockName(filename string) string {
	return filename + ".lock"
}
