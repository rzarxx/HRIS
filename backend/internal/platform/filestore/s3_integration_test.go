package filestore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
)

// TestSeaweedFSIntegration runs only when an explicit test endpoint is configured.
// Use a dedicated test bucket; the test only creates and removes its own objects.
func TestSeaweedFSIntegration(t *testing.T) {
	endpoint := os.Getenv("HRIS_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("HRIS_TEST_S3_ENDPOINT belum diisi")
	}
	cfg := S3Config{
		Endpoint: endpoint, Bucket: os.Getenv("HRIS_TEST_S3_BUCKET"),
		Region: "us-east-1", AccessKey: os.Getenv("HRIS_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("HRIS_TEST_S3_SECRET_KEY"),
	}
	cipher, err := fieldcrypt.New(bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewS3Store(cfg, cipher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	createdBucket := os.Getenv("HRIS_TEST_S3_CREATE_BUCKET") == "1"
	adminClient := store.client
	if createdBucket {
		if adminAccess := os.Getenv("HRIS_TEST_S3_ADMIN_ACCESS_KEY"); adminAccess != "" {
			adminCfg := cfg
			adminCfg.AccessKey = adminAccess
			adminCfg.SecretKey = os.Getenv("HRIS_TEST_S3_ADMIN_SECRET_KEY")
			adminStore, err := NewS3Store(adminCfg, cipher)
			if err != nil {
				t.Fatal(err)
			}
			adminClient = adminStore.client
		}
		if _, err := adminClient.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
			t.Fatalf("buat bucket uji: %v", err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanupCancel()
			if _, err := adminClient.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
				t.Errorf("hapus bucket uji: %v", err)
			}
		})
	}
	if err := store.Check(ctx); err != nil {
		t.Fatalf("HeadBucket: %v", err)
	}
	key := fmt.Sprintf("integration/%d.bin", time.Now().UnixNano())
	plain := []byte("HRIS SeaweedFS encrypted integration payload")
	if err := store.Put(ctx, key, plain); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := store.Delete(cleanupCtx, key); err != nil {
			t.Errorf("hapus objek uji: %v", err)
		}
	})
	raw, err := store.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("GetObject mentah: %v", err)
	}
	ciphertext, err := io.ReadAll(raw.Body)
	_ = raw.Body.Close()
	if err != nil || bytes.Contains(ciphertext, plain) || bytes.Equal(ciphertext, plain) {
		t.Fatalf("objek tidak terenkripsi atau gagal dibaca: %v", err)
	}
	got, err := store.Get(ctx, key)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Get/dekripsi: %v", err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, key); err == nil {
		t.Fatal("Get setelah Delete masih berhasil")
	}
	// Migrasi menyalin ciphertext lama apa adanya; API tetap harus bisa membukanya.
	migratedKey := fmt.Sprintf("profile/integration/%d.jpg", time.Now().UnixNano())
	legacyCiphertext, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(cfg.Bucket), Key: aws.String(migratedKey), Body: bytes.NewReader(legacyCiphertext),
	}); err != nil {
		t.Fatalf("unggah ciphertext lama: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = store.Delete(cleanupCtx, migratedKey)
	})
	got, err = store.Get(ctx, migratedKey)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("ciphertext hasil migrasi tidak dapat dibuka: %v", err)
	}

	if os.Getenv("HRIS_TEST_S3_CHECK_AUTH") == "1" {
		badCfg := cfg
		badCfg.SecretKey += "-wrong"
		badStore, err := NewS3Store(badCfg, cipher)
		if err != nil {
			t.Fatal(err)
		}
		if err := badStore.Check(ctx); err == nil {
			t.Fatal("kredensial salah masih dapat mengakses bucket")
		}
	}
}
