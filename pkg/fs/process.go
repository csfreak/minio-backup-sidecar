/*
 * Minio Backup Sidecar
 * Copyright 2023 Jason Ross.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fs

import (
	"context"
	"sync"

	"k8s.io/klog/v2"
)

var waitGroup sync.WaitGroup

func (c *Config) Process(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	go setupSignalNotify(cancel)

	for _, p := range c.Paths {
		doConfigPath(p, ctx)
	}

	waitGroup.Wait()
}

func doConfigPath(p *fsPath, ctx context.Context) {
	klog.V(4).InfoS("processing path", "fsPath", p)

	if p.Watch {
		startNewWatcher(p, ctx, &waitGroup)
	} else {
		waitGroup.Add(1)

		go func() {
			f, err := fileList(p.Path)
			if err != nil {
				klog.ErrorS(err, "unable to process path", "path", p.Path)
				return
			}

			for _, file := range *f {
				callUpload(p, file, ctx)
			}

			waitGroup.Done()
		}()
	}
}
