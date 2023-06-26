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
	"errors"
	"sync"

	"github.com/fsnotify/fsnotify"
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
		klog.V(3).InfoS("start watching path", "path", p.Path)
		doWatchPath(p, ctx)
	} else {
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

func doWatchPath(p *fsPath, ctx context.Context) {
	if !p.Watch {
		klog.ErrorS(errors.New("invalid fsPath. Watch is False"), "unable to watch fsPath", "fsPath", p)
		return
	}

	waitGroup.Add(1)

	go func() {
		watchCtx, watchCancel := context.WithCancel(ctx)

		watchPaths := []string{p.Path}

		if p.Recursive {
			klog.V(4).InfoS("watching path recursively", "path", p.Path)

			dirs, err := recursiveDirList(p.Path)
			if err != nil {
				klog.ErrorS(err, "unable to recurse path", "path", p.Path)
			}

			if dirs != nil {
				watchPaths = *dirs
			} else {
				klog.Warning("no paths found to watch", "path", p.Path)
			}
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			klog.ErrorS(err, "unable to setup watcher")
			watchCancel()
		}

		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						klog.V(2).ErrorS(err, "watcher closed")
						watchCancel()

						return
					}

					klog.V(4).InfoS("watcher received event", "event", event, "fsPath", p)

					switch {
					case event.Has(fsnotify.Create):
						if err := checkDir(event.Name); err == nil {
							klog.V(4).InfoS("adding new directory", "dir", event.Name, "path", p)
							addDirToWatcher(watcher, event.Name)
						} else if p.Events.Create {
							callUpload(p, event.Name, watchCtx)
						}

					case event.Has(fsnotify.Write):
						if p.Events.Create {
							callUpload(p, event.Name, watchCtx)
						}

					case event.Has(fsnotify.Remove):
						if p.Events.Remove {
							callDelete(p, event.Name, watchCtx)
						}

						checkWatcher(watcher, watchCancel)
					}

				case err, ok := <-watcher.Errors:
					klog.V(2).ErrorS(err, "watch error")

					if !ok {
						watchCancel()
						return
					}
				}
			}
		}()

		addDirToWatcher(watcher, watchPaths...)
		checkWatcher(watcher, watchCancel)

		<-watchCtx.Done()
		klog.V(2).InfoS("context canceled", "fsPath", p)
		watcher.Close()
		waitGroup.Done()
	}()
}

func addDirToWatcher(watcher *fsnotify.Watcher, paths ...string) {
	for _, p := range paths {
		klog.V(4).InfoS("add inotify watcher", "path", p)

		err := watcher.Add(p)
		if err != nil {
			klog.ErrorS(err, "unable to setup watcher")
		}
	}
}

func checkWatcher(watcher *fsnotify.Watcher, cancelFunc context.CancelFunc) {
	klog.V(4).InfoS("check watcher", watcher.WatchList())

	watch_count := len(watcher.WatchList())
	klog.V(4).InfoS("check watcher", "count", watch_count)

	if watch_count == 0 {
		klog.V(2).Info("no watchers running")
		cancelFunc()
	}
}
