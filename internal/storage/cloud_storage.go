package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"cloud.google.com/go/storage"
)

const (
	// BucketName はCloud Storageのバケット名
	BucketName = "GCS_BUCKET"
)

// UserKey はメールアドレスからユーザー固有のキーを生成する
func UserKey(email string) string {
	hash := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x", hash[:8])
}

// CSVObjectPath はユーザーのCSVオブジェクトパスを返す
func CSVObjectPath(email string) string {
	return fmt.Sprintf("users/%s/scores.csv", UserKey(email))
}

// DownloadCSV はCloud StorageからCSVをローカルファイルにダウンロードする
// ファイルが存在しない場合はfalseを返す
func DownloadCSV(email, localPath string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	objPath := CSVObjectPath(email)
	reader, err := client.Bucket(BucketName).Object(objPath).NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			log.Printf("[INFO] No existing CSV found for user")
			return false, nil
		}
		return false, fmt.Errorf("failed to read from GCS: %w", err)
	}
	defer reader.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to create local file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return false, fmt.Errorf("failed to download CSV: %w", err)
	}

	log.Printf("[INFO] Downloaded existing CSV from GCS")
	return true, nil
}

// UploadCSV はローカルのCSVファイルをCloud Storageにアップロードする
func UploadCSV(email, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer f.Close()

	objPath := CSVObjectPath(email)
	writer := client.Bucket(BucketName).Object(objPath).NewWriter(ctx)
	writer.ContentType = "text/csv"

	if _, err := io.Copy(writer, f); err != nil {
		return fmt.Errorf("failed to upload CSV: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	log.Printf("[INFO] Uploaded CSV to GCS")
	return nil
}
