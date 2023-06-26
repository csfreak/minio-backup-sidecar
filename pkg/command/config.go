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
	"strings"

	"github.com/spf13/viper"
)

func initConfig() {
	// Setup Viper
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "__", "-", "_"))
	viper.SetEnvPrefix("conf")
	viper.AutomaticEnv()
	viper.AllowEmptyEnv(true)

	// Setup Global Defaults
	viper.SetDefault("watch", true)
	viper.SetDefault("recursive", false)
	viper.SetDefault("watch-events", []string{"Create", "Write"})
	viper.SetDefault("delete-on-success", false)
}
