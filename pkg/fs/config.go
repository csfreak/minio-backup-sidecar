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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/csfreak/minio-backup-sidecar/pkg/config"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

type Event string

const (
	CreateEvent Event = "Create"
	WriteEvent  Event = "Write"
	RemoveEvent Event = "Remove"
)

type Config struct {
	Paths []*fsPath
}

type fsPath struct {
	Watch       bool    // Watch Path or process once (Defaults to true)
	Recursive   bool    // Watch Path Recursively (only applies if Path is a Directory) (Defaults to false)
	Path        string  // Path of File or Directory
	Events      []Event // What Events to Watch (Create, Write, Update) (only applies if Watch = True)
	Destination config.Destination
}

func New() (*Config, error) {
	c := &Config{}

	if viper.IsSet("path") {
		for _, p := range viper.GetStringSlice("path") {
			fsp, err := newPath(p)
			if err != nil {
				klog.ErrorS(err, "error processing path")
			} else {
				if viper.IsSet("destination.name") {
					if fsp.Destination.Name != "" {
						klog.Warningf("setting destination.name for directory %s may result in files being overwritten", fsp.Path)
					}
					fsp.Destination.Name = viper.GetString("destination.name")
				}
				if viper.IsSet("destination.path") {
					fsp.Destination.Path = viper.GetString("destination.path")
				}
				if viper.IsSet("destination.type") {
					fsp.Destination.Path = viper.GetString("destination.type")
				}
				c.Paths = append(c.Paths, fsp)
			}
		}
	}

	for i := 0; viper.IsSet(fmt.Sprintf("files.%d.path", i)); i++ {
		fsp, err := newPath(viper.GetString(fmt.Sprintf("files.%d.path", i)))
		if err != nil {
			klog.ErrorS(err, "error processing path")
		} else {
			if viper.IsSet(fmt.Sprintf("files.%d.watch", i)) {
				fsp.Watch = viper.GetBool(fmt.Sprintf("files.%d.watch", i))
			}
			if viper.IsSet(fmt.Sprintf("files.%d.recursive", i)) {
				fsp.Watch = viper.GetBool(fmt.Sprintf("files.%d.recursive", i))
			}
			if viper.IsSet(fmt.Sprintf("files.%d.events", i)) {
				events, err := ParseEvents(viper.GetStringSlice(fmt.Sprintf("files.%d.events", i)))
				if err != nil {
					klog.ErrorS(err, "error processing path")
					continue
				}
				fsp.Events = events
			}
			if viper.IsSet("files.%d.destination.name") {
				if fsp.Destination.Name != "" {
					klog.Warningf("setting destination.name for directory %s may result in files being overwritten", fsp.Path)
				}
				fsp.Destination.Name = viper.GetString(fmt.Sprintf("files.%d.destination.name", i))
			}
			if viper.IsSet(fmt.Sprintf("files.%d.destination.path", i)) {
				fsp.Destination.Path = viper.GetString(fmt.Sprintf("files.%d.destination.name", i))
			}
			if viper.IsSet(fmt.Sprintf("files.%d.destination.type", i)) {
				fsp.Destination.Type = viper.GetString(fmt.Sprintf("files.%d.destination.name", i))
			}
			c.Paths = append(c.Paths, fsp)
		}
	}

	if len(c.Paths) == 0 {
		return nil, errors.New("no paths found")
	}

	return c, nil
}

func newPath(p string) (*fsPath, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("unable to process path %s: %w", p, err)
	}

	var (
		filename string
		filepath string
	)

	if info.IsDir() {
		filename = ""
		filepath = p
	} else {
		filepath, filename = path.Split(p)
	}

	events, err := ParseEvents(viper.GetStringSlice("watch-events"))
	if err != nil {
		return nil, err
	}

	return &fsPath{
		Watch:     viper.GetBool("watch"),
		Recursive: viper.GetBool("recursive"),
		Path:      p,
		Events:    events,
		Destination: config.Destination{
			Name: filename,
			Path: filepath,
		},
	}, nil
}

func ParseEvent(e string) (Event, error) {
	switch strings.ToLower(e) {
	case "create":
		return CreateEvent, nil
	case "write", "update":
		return WriteEvent, nil
	case "remove", "delete":
		return RemoveEvent, nil
	}

	return Event(""), fmt.Errorf("unable to parse %s into %T", e, Event(""))
}

func ParseEvents(eventNames []string) ([]Event, error) {
	events := []Event{}

	for _, name := range eventNames {
		e, err := ParseEvent(name)
		if err != nil {
			return events, err
		}

		events = append(events, e)
	}

	return events, nil
}

func (e Event) String() string {
	return string(e)
}
