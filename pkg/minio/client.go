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

package minio

import (
	"context"
	"fmt"
	"path"

	"github.com/csfreak/minio-backup-sidecar/pkg/config"
	mc "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

type MinioClient interface {
	newClient() error
	makeBucket(ctx context.Context) error
	UploadFile(file string, ctx context.Context) error
	UploadFileWithDestination(file string, dest config.Destination, ctx context.Context) error
}

type minioConfig struct {
	client *mc.Client
	bucket string
}

func New(ctx context.Context) (MinioClient, error) {
	klog.V(3).Info("configuring minio")

	c := &minioConfig{}

	err := c.newClient()
	if err != nil {
		return nil, fmt.Errorf("unable to initialize minio client: %w", err)
	}

	err = c.makeBucket(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to find or create minio bucket: %w", err)
	}

	return c, nil
}

func (c *minioConfig) newClient() error {
	klog.V(4).Info("creating new client")

	if !viper.IsSet("minio.endpoint") {
		klog.V(3).Info("minio.endpoint not set")
		return fmt.Errorf("minio.endpoint must be set")
	}

	if !viper.IsSet("minio.access-key-id") {
		klog.V(3).Info("minio.access-key-id not set")
		return fmt.Errorf("minio.access-key-id must be set")
	}

	if !viper.IsSet("minio.access-key-secret") {
		klog.V(3).Info("minio.access-key-secret not set")
		return fmt.Errorf("minio.access-key-secret must be set")
	}

	client, err := mc.New(viper.GetString("minio.endpoint"), &mc.Options{
		Creds:  credentials.NewStaticV4(viper.GetString("minio.access-key-id"), viper.GetString("minio.access-key-secret"), ""),
		Secure: viper.GetBool("minio.secure"),
	})
	if err != nil {
		klog.V(3).ErrorS(err, "unable to create minio client")
		return fmt.Errorf("unable to create minio client: %w", err)
	}

	klog.V(3).Info("created minio client")

	c.client = client

	return nil
}

func (c *minioConfig) makeBucket(ctx context.Context) error {
	klog.V(3).Info("making bucket")

	if !viper.IsSet("minio.bucket") {
		return fmt.Errorf("minio.bucket must be set")
	}

	bucket := viper.GetString("minio.bucket")
	o := mc.MakeBucketOptions{}

	if viper.IsSet("minio.region") {
		o.Region = viper.GetString("minio.region")
	}

	klog.V(4).InfoS("bucket params", "name", bucket, "options", o)

	err := c.client.MakeBucket(ctx, bucket, o)
	if err != nil {
		klog.V(4).ErrorS(err, "unable to create bucket")
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := c.client.BucketExists(ctx, bucket)
		if errBucketExists == nil && exists {
			klog.Infof("bucket %s already exists, using it", bucket)
		} else {
			klog.V(3).ErrorS(errBucketExists, "bucket does not exist to cannot check")
			return fmt.Errorf("unable to create bucket: %w", err)
		}
	} else {
		klog.Infof("Successfully created %s", bucket)
	}

	c.bucket = bucket

	if viper.IsSet("minio.retention") {
		klog.V(3).Info("setting bucket retention")

		lc := lifecycle.NewConfiguration()
		lc.Rules = append(lc.Rules, lifecycle.Rule{Status: "Enabled", Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(viper.GetInt("minio.retention"))}})

		klog.V(4).InfoS("bucket lifecycle", "lifecycle.Configuration", lc)

		err = c.client.SetBucketLifecycle(ctx, bucket, lc)
		if err != nil {
			return fmt.Errorf("unable to set retention policy: %w", err)
		}

		klog.Infof("Set bucket retention policy to %d days", viper.GetInt("minio.retention"))
	}

	return nil
}

func (c *minioConfig) UploadFile(file string, ctx context.Context) error {
	_, filename := path.Split(file)
	return c.UploadFileWithDestination(file, config.Destination{Name: filename}, ctx)
}

func (c *minioConfig) UploadFileWithDestination(file string, dest config.Destination, ctx context.Context) error {
	var objName string

	if dest.Name == "" {
		_, filename := path.Split(file)
		dest.Name = filename
	}

	if dest.Path != "" {
		objName = path.Join(dest.Path, dest.Name)
	} else {
		objName = dest.Name
	}

	klog.V(2).InfoS("uploading file", "file", "file", "destination", "objName", "content-type", dest.Type)

	info, err := c.client.FPutObject(ctx, c.bucket, objName, file, mc.PutObjectOptions{ContentType: dest.Type})
	if err != nil {
		return fmt.Errorf("unable to put %s: %w", objName, err)
	}

	klog.Infof("successfully uploaded %s of size %d to %s", objName, info.Size, c.bucket)

	return nil
}
