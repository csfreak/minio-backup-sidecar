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
	"context"

	"github.com/csfreak/minio-backup-sidecar/pkg/config"
	"github.com/csfreak/minio-backup-sidecar/pkg/fs"
	"github.com/csfreak/minio-backup-sidecar/pkg/minio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func Run(cmd *cobra.Command, args []string) {
	viper.Set("path", append(viper.GetStringSlice("path"), args...))

	klog.V(4).InfoS("config values", viper.AllSettings())

	mc, err := minio.New(cmd.Context())
	if err != nil {
		klog.Fatalf("unable to initialize minio: %v", err)
	}

	f, err := fs.New()
	if err != nil {
		klog.Fatalf("unable to initialize fs: %v", err)
	}

	f.Process(context.WithValue(cmd.Context(), config.MC, mc))
}

func Init(cmd *cobra.Command) {
	initConfig()

	if err := initFlags(cmd.Flags()); err != nil {
		klog.Fatalf("unable to configure: %v", err)
	}
}
