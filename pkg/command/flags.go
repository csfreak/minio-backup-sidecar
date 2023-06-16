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

package command

import (
	"flag"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

var klogVisibleFlags = []string{"v"}

func initFlags(flags *pflag.FlagSet) error {
	flags.AddFlagSet(initKlogFlags())

	flags.String("minio.endpoint", "", "Hostname of Minio Endpoint")
	flags.String("minio.access-key-id", "", "Minio Access Key ID")
	flags.String("minio.access-key-secret", "", "Minio Access Key Secret")
	flags.String("minio.region", "", "Minio Region")
	flags.String("minio.bucket", "", "Minio Bucket Name")
	flags.Int("minio.retention", 0, "Set Minio Lifecycle In Days")
	flags.Bool("minio.secure", true, "Use SSL/TLS for Minio Client")

	flags.BoolP("watch", "w", true, "Watch path for changes")
	flags.BoolP("recursive", "r", false, "Watch directory paths recursively")
	flags.StringArray("path", []string{}, "Path to watch")
	flags.StringArray("watch-events", []string{"Create", "Write"}, "Events to Watch")
	flags.String("destination.name", "", "Object Name in bucket")
	flags.String("destination.path", "", "Object Path in bucket")
	flags.String("destination.type", "", "Object MIME type")

	return viper.BindPFlags(flags)
}

func initKlogFlags() *pflag.FlagSet {
	goFlagSet := &flag.FlagSet{}
	klog.InitFlags(goFlagSet)

	klogFlagSet := &pflag.FlagSet{}

	klogFlagSet.AddGoFlagSet(goFlagSet)

	if err := klogFlagSet.Set("logtostderr", "true"); err != nil {
		klog.V(4).ErrorS(err, "error setting up klog flag")
	}

	// We do not want these flags to show up in --help
	// These MarkHidden calls must be after the lines above
	klogFlagSet.VisitAll(func(f *pflag.Flag) {
		if err := klogFlagSet.MarkHidden(f.Name); err != nil {
			klog.V(4).ErrorS(err, "error setting up klog flags")
		}
	})

	for _, name := range klogVisibleFlags {
		klogFlagSet.Lookup(name).Hidden = false
	}

	return klogFlagSet
}
