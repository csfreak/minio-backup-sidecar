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
	"log"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

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

		var watchPaths []string

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
		} else {
			watchPaths = []string{p.Path}
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

					for _, e := range p.Events {
						switch e {
						case CreateEvent:
							if event.Has(fsnotify.Create) {
								callUpload(p, event.Name, watchCtx)
							}
						case WriteEvent:
							if event.Has(fsnotify.Write) {
								callUpload(p, event.Name, watchCtx)
							}
						case RemoveEvent:
							if event.Has(fsnotify.Remove) {
								callDelete(p, event.Name, watchCtx)
							}
						}
					}

					if event.Has(fsnotify.Write) {
						log.Println("modified file:", event.Name)
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

		for _, watch := range watchPaths {
			klog.V(4).InfoS("add inotify watcher to path", "path", p.Path)

			err = watcher.Add(watch)
			if err != nil {
				klog.ErrorS(err, "unable to setup watcher")
			}
		}

		if len(watcher.WatchList()) == 0 {
			klog.V(2).ErrorS(err, "no watchers running")
			watchCancel()
		}

		<-watchCtx.Done()
		klog.V(2).InfoS("context canceled", "fsPath", p)
		watcher.Close()
		waitGroup.Done()
	}()
}
