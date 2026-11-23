// Package filestore provides private, encrypted object storage for application files.
package filestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const maxObjectBytes = 64 << 20

type Cipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

type Store interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

// S3Store stores ciphertext in a private bucket. Plaintext never leaves the API process.
type S3Store struct {
	client *s3.Client
	bucket string
	cipher Cipher
}

func NewS3Store(cfg S3Config, cipher Cipher) (*S3Store, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cipher == nil {
		return nil, errors.New("konfigurasi S3 dan kunci enkripsi wajib diisi")
	}
	client := s3.NewFromConfig(aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		HTTPClient:  &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &S3Store{client: client, bucket: cfg.Bucket, cipher: cipher}, nil
}

func validKey(key string) bool {
	return key != "" && key != ".." && !strings.HasPrefix(key, "../") &&
		!strings.HasPrefix(key, "/") && !strings.Contains(key, "\\") &&
		!strings.Contains(key, "//") && path.Clean(key) == key && key != "." &&
		!strings.ContainsRune(key, '\x00')
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	if !validKey(key) {
		return fmt.Errorf("kunci berkas tidak valid: %q", key)
	}
	if len(data) > maxObjectBytes-1024 {
		return errors.New("berkas terlalu besar")
	}
	enc, err := s.cipher.Encrypt(data)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
		Body: bytes.NewReader(enc), ContentType: aws.String("application/octet-stream"),
	})
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	if !validKey(key) {
		return nil, fmt.Errorf("kunci berkas tidak valid: %q", key)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	if out.ContentLength != nil && *out.ContentLength > maxObjectBytes {
		return nil, errors.New("berkas terlalu besar")
	}
	enc, err := io.ReadAll(io.LimitReader(out.Body, maxObjectBytes+1))
	if err != nil {
		return nil, err
	}
	if len(enc) > maxObjectBytes {
		return nil, errors.New("berkas terlalu besar")
	}
	return s.cipher.Decrypt(enc)
}

// Delete is idempotent, including for objects already removed by a prior retention run.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("kunci berkas tidak valid: %q", key)
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return nil
	}
	return err
}

func (s *S3Store) Check(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}

// Probe verifies bucket read and write permissions without touching user files.
func (s *S3Store) Probe(ctx context.Context) error {
	if err := s.Check(ctx); err != nil {
		return err
	}
	key := "probes/" + uuid.NewString()
	if err := s.Put(ctx, key, []byte("hris-s3-probe")); err != nil {
		return err
	}
	defer s.Delete(context.Background(), key)
	got, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, []byte("hris-s3-probe")) {
		return errors.New("hasil baca uji S3 tidak sesuai")
	}
	return s.Delete(ctx, key)
}
