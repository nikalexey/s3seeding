// Package s3client is a thin wrapper around minio-go for benchmark operations:
// connecting, Put/Get/Delete and checking/creating the bucket.
package s3client

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"s3bench/internal/config"
)

type Client struct {
	mc       *minio.Client
	bucket   string
	Name     string
	partSize uint64
}

func New(ctx context.Context, s config.Storage, partSizeMB int) (*Client, error) {
	endpoint, secure, err := parseEndpoint(s.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("storage %q: %w", s.Name, err)
	}
	tr, err := minio.DefaultTransport(secure)
	if err != nil {
		return nil, err
	}
	if s.InsecureSkipVerify && tr.TLSClientConfig != nil {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	opts := &minio.Options{
		Creds:     credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""),
		Secure:    secure,
		Transport: tr,
	}
	if s.PathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}
	mc, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("storage %q: %w", s.Name, err)
	}
	c := &Client{
		mc:       mc,
		bucket:   s.Bucket,
		Name:     s.Name,
		partSize: uint64(partSizeMB) * 1024 * 1024,
	}
	if err := c.ensureBucket(ctx, s); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) ensureBucket(ctx context.Context, s config.Storage) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	exists, err := c.mc.BucketExists(cctx, c.bucket)
	if err != nil {
		return fmt.Errorf("storage %q: bucket check: %w", s.Name, wrapErr("bucket-exists", c.bucket, err))
	}
	if exists {
		return nil
	}
	return fmt.Errorf("storage %q: bucket %q does not exist", s.Name, c.bucket)
}

func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{
		PartSize:    c.partSize,
		ContentType: "application/octet-stream",
	})
	return wrapErr("put", key, err)
}

func (c *Client) Get(ctx context.Context, key string, w io.Writer) (int64, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return 0, wrapErr("get", key, err)
	}
	defer obj.Close()
	n, err := io.Copy(w, obj)
	return n, wrapErr("get", key, err)
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return wrapErr("delete", key, c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{}))
}

func parseEndpoint(ep string) (host string, secure bool, err error) {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	if strings.Contains(ep, "://") {
		u, perr := url.Parse(ep)
		if perr != nil {
			return "", false, fmt.Errorf("parse endpoint %q: %w", ep, perr)
		}
		switch u.Scheme {
		case "https":
			return u.Host, true, nil
		case "http":
			return u.Host, false, nil
		default:
			return "", false, fmt.Errorf("endpoint %q: unsupported scheme %q", ep, u.Scheme)
		}
	}
	return ep, true, nil
}
