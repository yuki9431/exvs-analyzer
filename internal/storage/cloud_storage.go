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

// BucketName は環境変数 GCS_BUCKET から取得するCloud Storageのバケット名
var BucketName = os.Getenv("GCS_BUCKET")

// UserKey はメールアドレスからユーザー固有のキーを生成する
func UserKey(email string) string {
	hash := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x", hash[:8])
}

// CSVObjectPath はユーザーのCSVオブジェクトパスを返す
func CSVObjectPath(email string) string {
	return fmt.Sprintf("users/%s/scores.csv", UserKey(email))
}

// downloadObject はGCSからオブジェクトをローカルファイルにダウンロードする
// オブジェクトが存在しない場合はfalseを返す
func downloadObject(objPath, localPath string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	reader, err := client.Bucket(BucketName).Object(objPath).NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
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
		return false, fmt.Errorf("failed to download from GCS: %w", err)
	}

	return true, nil
}

// uploadObject はローカルファイルをGCSにアップロードする
func uploadObject(objPath, localPath, contentType string) error {
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

	writer := client.Bucket(BucketName).Object(objPath).NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, f); err != nil {
		return fmt.Errorf("failed to upload to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

// DownloadCSV はCloud StorageからCSVをローカルファイルにダウンロードする
// ファイルが存在しない場合はfalseを返す
func DownloadCSV(email, localPath string) (bool, error) {
	found, err := downloadObject(CSVObjectPath(email), localPath)
	if err != nil {
		return false, err
	}
	if found {
		log.Printf("[INFO] Downloaded existing CSV from GCS")
	} else {
		log.Printf("[INFO] No existing CSV found for user")
	}
	return found, nil
}

// DownloadCSVByKey はユーザーキーを使ってCloud StorageからCSVをダウンロードする
func DownloadCSVByKey(userKey, localPath string) (bool, error) {
	return downloadObject(fmt.Sprintf("users/%s/scores.csv", userKey), localPath)
}

// TagPartnersObjectPath はユーザーのタッグ相方JSONオブジェクトパスを返す
func TagPartnersObjectPath(email string) string {
	return fmt.Sprintf("users/%s/tag_partners.json", UserKey(email))
}

// DownloadTagPartners はCloud Storageからタッグ相方JSONをローカルファイルにダウンロードする
// ファイルが存在しない場合はfalseを返す
func DownloadTagPartners(email, localPath string) (bool, error) {
	found, err := downloadObject(TagPartnersObjectPath(email), localPath)
	if err != nil {
		return false, err
	}
	if found {
		log.Printf("[INFO] Downloaded existing tag partners from GCS")
	} else {
		log.Printf("[INFO] No existing tag partners found for user")
	}
	return found, nil
}

// UploadCSV はローカルのCSVファイルをCloud Storageにアップロードする
func UploadCSV(email, localPath string) error {
	if err := uploadObject(CSVObjectPath(email), localPath, "text/csv"); err != nil {
		return err
	}
	log.Printf("[INFO] Uploaded CSV to GCS")
	return nil
}

// UploadTagPartners はローカルのタッグ相方JSONファイルをCloud Storageにアップロードする
func UploadTagPartners(email, localPath string) error {
	if err := uploadObject(TagPartnersObjectPath(email), localPath, "application/json"); err != nil {
		return err
	}
	log.Printf("[INFO] Uploaded tag partners to GCS")
	return nil
}
