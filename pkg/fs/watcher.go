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

type watcher struct {
	p        *fsPath
	_ctx     context.Context
	_cancel  context.CancelFunc
	_wg      *sync.WaitGroup
	_watcher *fsnotify.Watcher
}

func startNewWatcher(p *fsPath, ctx context.Context, wg *sync.WaitGroup) {
	klog.V(3).InfoS("start watching path", "path", p.Path)

	if !p.Watch {
		klog.ErrorS(errors.New("invalid fsPath. Watch is False"), "unable to watch fsPath", "fsPath", p)
		return
	}

	w := &watcher{
		p:   p,
		_wg: wg,
	}

	w._ctx, w._cancel = context.WithCancel(ctx)

	_watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.ErrorS(err, "unable to setup watcher")
		w._cancel()
	}

	w._watcher = _watcher

	w.startWatcher()

	watchPaths := []string{w.p.Path}

	if w.p.Recursive {
		klog.V(4).InfoS("watching path recursively", "path", w.p.Path)

		dirs, err := recursiveDirList(w.p.Path)
		if err != nil {
			klog.ErrorS(err, "unable to recurse path", "path", w.p.Path)
		}

		if dirs != nil {
			watchPaths = *dirs
		} else {
			klog.Warning("no paths found to watch", "path", w.p.Path)
		}
	}

	w.addDir(watchPaths...)
	w.checkWatcher()
}

func (w *watcher) startWatcher() {
	w._wg.Add(1)

	go func() {
		w.startWatchLoop()

		<-w._ctx.Done()
		klog.V(2).InfoS("context canceled", "fsPath", w.p)
		w._watcher.Close()
		waitGroup.Done()
	}()
}

func (w *watcher) startWatchLoop() {
	go func() {
		for {
			select {
			case event, ok := <-w._watcher.Events:
				if !ok {
					klog.V(2).InfoS("watcher closed", "path", w.p.Path)
					w._cancel()

					return
				}

				klog.V(4).InfoS("watcher received event", "event", event, "path", w.p.Path)

				switch {
				case event.Has(fsnotify.Create):
					if err := checkDir(event.Name); err == nil {
						klog.V(4).InfoS("adding new directory", "dir", event.Name, "path", w.p.Path)
						w.addDir(event.Name)
					} else if w.p.Events.Create {
						callUpload(w.p, event.Name, w._ctx)
					}

				case event.Has(fsnotify.Write):
					if w.p.Events.Create {
						callUpload(w.p, event.Name, w._ctx)
					}

				case event.Has(fsnotify.Remove):
					if w.p.Events.Remove {
						callDelete(w.p, event.Name, w._ctx)
					}

					w.checkWatcher()
				}

			case err, ok := <-w._watcher.Errors:
				klog.V(2).ErrorS(err, "watch error")

				if !ok {
					w._cancel()
					return
				}
			}
		}
	}()
}

func (w *watcher) addDir(paths ...string) {
	for _, p := range paths {
		klog.V(4).InfoS("add inotify watcher", "path", w.p.Path, "new", p)

		err := w._watcher.Add(p)
		if err != nil {
			klog.ErrorS(err, "unable to setup watcher", "path", w.p.Path, "new", p)
		}
	}
}

func (w *watcher) checkWatcher() {
	klog.V(4).InfoS("check watcher", w._watcher.WatchList())

	watch_count := len(w._watcher.WatchList())
	klog.V(4).InfoS("check watcher", "count", watch_count)

	if watch_count == 0 {
		klog.V(2).Info("no watchers running")
		w._cancel()
	}
}
