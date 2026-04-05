// cloud_storage_test.go — internal/storage パッケージのユニットテスト（GCS非依存部分）
//
// テスト対象と観点:
//   - UserKey: メールアドレス→SHA256ハッシュ先頭8バイト16進数。同じ入力で同じ出力か、異なる入力で異なる出力か
//   - CSVObjectPath: UserKeyを使ったGCSオブジェクトパスの組み立て
//
// GCSに接続する DownloadCSV/UploadCSV はテスト対象外。
//
// 実行方法:
//   make test
package storage

import (
	"testing"
)

func TestUserKey(t *testing.T) {
	// 同じ入力は同じ結果を返す
	key1 := UserKey("test@example.com")
	key2 := UserKey("test@example.com")
	if key1 != key2 {
		t.Errorf("same input should produce same key: %q != %q", key1, key2)
	}

	// 異なる入力は異なる結果を返す
	key3 := UserKey("other@example.com")
	if key1 == key3 {
		t.Errorf("different input should produce different key: both %q", key1)
	}

	// 16進数16文字（8バイト）
	if len(key1) != 16 {
		t.Errorf("key length = %d, want 16", len(key1))
	}
}

func TestCSVObjectPath(t *testing.T) {
	path := CSVObjectPath("test@example.com")
	want := "users/" + UserKey("test@example.com") + "/scores.csv"
	if path != want {
		t.Errorf("CSVObjectPath = %q, want %q", path, want)
	}
}
