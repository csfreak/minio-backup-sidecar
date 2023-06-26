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

type Config struct {
	Paths []*fsPath
}

type Events struct {
	Create bool
	Write  bool
	Remove bool
}

type fsPath struct {
	DeleteOnSuccess bool    // Delete files after successful upload
	Watch           bool    // Watch Path or process once (Defaults to true)
	Recursive       bool    // Watch Path Recursively (only applies if Path is a Directory) (Defaults to false)
	Path            string  // Path of File or Directory
	Events          *Events // What Events to Watch (Create, Write, Remove) (only applies if Watch = True)
	Destination     config.Destination
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
				fsp.Recursive = viper.GetBool(fmt.Sprintf("files.%d.recursive", i))
			}
			if viper.IsSet(fmt.Sprintf("files.%d.events", i)) {
				events, err := ParseEvents(viper.GetStringSlice(fmt.Sprintf("files.%d.events", i)))
				if err != nil {
					klog.ErrorS(err, "error processing path")
					continue
				}
				fsp.Events = events
			}
			if viper.IsSet(fmt.Sprintf("files.%d.delete-on-success", i)) {
				fsp.DeleteOnSuccess = viper.GetBool(fmt.Sprintf("files.%d.delete-on-success", i))
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

	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
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
		Watch:           viper.GetBool("watch"),
		Recursive:       viper.GetBool("recursive"),
		DeleteOnSuccess: viper.GetBool("delete-on-success"),
		Path:            p,
		Events:          events,
		Destination: config.Destination{
			Name: filename,
			Path: filepath,
		},
	}, nil
}

func (e *Events) setEvent(name string) error {
	switch strings.ToLower(name) {
	case "create":
		e.Create = true
	case "write", "update":
		e.Write = true
	case "remove", "delete":
		e.Remove = true
	default:
		return fmt.Errorf("unable to parse event %s", name)
	}

	return nil
}

func newEvents() *Events {
	return &Events{
		Create: false,
		Write:  false,
		Remove: false,
	}
}

func ParseEvents(eventNames []string) (*Events, error) {
	e := newEvents()
	for _, name := range eventNames {
		err := e.setEvent(name)
		if err != nil {
			return e, err
		}
	}

	return e, nil
}

func (c *Config) validate() error {
	for _, p := range c.Paths {
		if p.Watch {
			if err := checkDir(p.Path); err != nil {
				if p.Recursive {
					return fmt.Errorf("cannot recursively watch non-directory file: %s", p.Path)
				}

				if p.DeleteOnSuccess {
					return fmt.Errorf("cannot use delete-on-success and watch on non-directory file: %s", p.Path)
				}
			}

			if !(p.Events.Create || p.Events.Write || p.Events.Remove) {
				return fmt.Errorf("cannot set watch without any events: %s", p.Path)
			}
		} else {
			p.Recursive = false
			p.DeleteOnSuccess = false
			p.Events = newEvents()
		}

		if p.DeleteOnSuccess && p.Events.Remove {
			return fmt.Errorf("cannot watch remove/delete events with delete-on-success: %s", p.Path)
		}
	}

	return nil
}
