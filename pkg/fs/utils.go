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
	"fmt"
	"os"
	"path"

	"github.com/csfreak/minio-backup-sidecar/pkg/config"
	"github.com/csfreak/minio-backup-sidecar/pkg/minio"
	"k8s.io/klog/v2"
)

func checkDir(p string) error {
	info, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("unable to process path %s: %w", p, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", p)
	}

	return nil
}

func recursiveDirList(p string) (*[]string, error) {
	if err := checkDir(p); err != nil {
		klog.V(3).ErrorS(err, "unable to process path", "path", "p")

		return nil, err
	}

	dirs := []string{p}

	fs, err := os.ReadDir(p)
	if err != nil {
		klog.V(3).ErrorS(err, "unable to process dir", "path", "p")
		return nil, fmt.Errorf("unable to process dir %s: %w", p, err)
	}

	for _, f := range fs {
		if f.IsDir() {
			d, err := recursiveDirList(path.Join(p, f.Name()))
			if err != nil {
				klog.V(3).ErrorS(err, "unable to process dir", "path", "p", "directory", f.Name())
				return &dirs, err
			}

			dirs = append(dirs, *d...)
		}
	}

	return &dirs, nil
}

func fileList(p string) (*[]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		klog.V(3).ErrorS(err, "unable to process path", "path", "p")
		return nil, fmt.Errorf("unable to process path %s: %w", p, err)
	}

	if !info.IsDir() {
		return &[]string{p}, nil
	}

	files := []string{}

	fs, err := os.ReadDir(p)
	if err != nil {
		klog.V(3).ErrorS(err, "unable to process dir", "path", "p")
		return nil, fmt.Errorf("unable to process dir %s: %w", p, err)
	}

	for _, f := range fs {
		if !f.IsDir() {
			files = append(files, path.Join(p, f.Name()))
		}
	}

	return &files, nil
}

func callUpload(p *fsPath, file string, ctx context.Context) {
	klog.V(2).InfoS("uploading file", "file", file)

	if err := ctx.Value(config.MC).(minio.MinioClient).UploadFileWithDestination(file, p.Destination, ctx); err != nil {
		klog.ErrorS(err, "failed upload", "file", file, "fsPath", p)
		return
	}

	if p.DeleteOnSuccess {
		if err := os.Remove(file); err != nil {
			klog.ErrorS(err, "failed to remove uploaded file", "file", file)
		}
	}
}

func callDelete(_ *fsPath, file string, _ context.Context) {
	klog.Info("delete called but not yet implemented", "file", file)
}
