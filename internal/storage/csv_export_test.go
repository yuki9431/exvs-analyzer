// csv_export_test.go — internal/storage パッケージのユニットテスト
//
// テスト対象と観点:
//   - scoreToRow: DatedScore→CSV行（22カラム）の変換が正しいか
//   - exportAllScoresCSV: ヘッダー+データ行のCSV出力
//   - GetLatestDatetime: CSV内の最新日時を取得。ファイル未存在時はゼロ値を返すか
//   - SaveAllScoresCSV: 新規作成（ヘッダー付き）と既存ファイルへの追記。空データで変化しないか
//
// テストデータはすべて架空の値。
// ファイル系テストは t.TempDir() で一時フォルダを使い、実データに影響しない。
//
// 実行方法:
//   make test
package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

func TestScoreToRow(t *testing.T) {
	d := model.DatedScore{
		PlayerNo: 1,
		Datetime: time.Date(2025, 1, 15, 20, 30, 0, 0, time.UTC),
		PlayerScore: model.PlayerScore{
			City:           "東京都",
			Name:           "テストユーザー",
			Win:            "WIN",
			MsName:         "ガンダム",
			MsImage:        "https://example.com/gundam.png",
			Point:          150,
			Kills:          3,
			Deaths:         1,
			Give_damage:    5000,
			Receive_damage: 2000,
			Ex_damage:      1000,
			Mastery:        "master",
			TeamName:       "テストチーム",
			TitleImage:     "https://example.com/title.png",
			TitleBadge:     "https://example.com/badge.png",
			ProfileLink:    "https://web.vsmobile.jp/exvs2ib/profile?param=abc123",
			ShuffleGrade:   "https://example.com/grade1.png",
			TeamGrade:      "https://example.com/grade2.png",
			ScoreRanking:   1,
			ShopName:       "テストゲームセンター",
		},
	}

	row := scoreToRow(d)

	if row[0] != "2025-01-15 20:30" {
		t.Errorf("datetime: got %q, want %q", row[0], "2025-01-15 20:30")
	}
	if row[1] != "1" {
		t.Errorf("playerNo: got %q, want %q", row[1], "1")
	}
	if row[2] != "東京都" {
		t.Errorf("city: got %q, want %q", row[2], "東京都")
	}
	if row[5] != "ガンダム" {
		t.Errorf("msName: got %q, want %q", row[5], "ガンダム")
	}
	if row[13] != "master" {
		t.Errorf("mastery: got %q, want %q", row[13], "master")
	}
	if row[14] != "テストチーム" {
		t.Errorf("teamName: got %q, want %q", row[14], "テストチーム")
	}
	if row[15] != "https://example.com/title.png" {
		t.Errorf("titleImage: got %q, want %q", row[15], "https://example.com/title.png")
	}
	if row[16] != "https://example.com/badge.png" {
		t.Errorf("titleBadge: got %q, want %q", row[16], "https://example.com/badge.png")
	}
	if row[17] != "https://web.vsmobile.jp/exvs2ib/profile?param=abc123" {
		t.Errorf("profileLink: got %q, want %q", row[17], "https://web.vsmobile.jp/exvs2ib/profile?param=abc123")
	}
	if row[18] != "https://example.com/grade1.png" {
		t.Errorf("shuffleGrade: got %q, want %q", row[18], "https://example.com/grade1.png")
	}
	if row[19] != "https://example.com/grade2.png" {
		t.Errorf("teamGrade: got %q, want %q", row[19], "https://example.com/grade2.png")
	}
	if row[20] != "1" {
		t.Errorf("scoreRanking: got %q, want %q", row[20], "1")
	}
	if row[21] != "テストゲームセンター" {
		t.Errorf("shopName: got %q, want %q", row[21], "テストゲームセンター")
	}
	if len(row) != 22 {
		t.Errorf("row length: got %d, want 22", len(row))
	}
}

func TestExportAllScoresCSV(t *testing.T) {
	ds := model.DatedScores{
		{
			PlayerNo: 1,
			Datetime: time.Date(2025, 1, 15, 20, 30, 0, 0, time.UTC),
			PlayerScore: model.PlayerScore{
				City: "東京都", Name: "テスト", Win: "WIN",
				MsName: "ガンダム", MsImage: "img.png",
				Point: 100, Kills: 2, Deaths: 1,
				Give_damage: 3000, Receive_damage: 1000, Ex_damage: 500,
			},
		},
	}

	var buf bytes.Buffer
	if err := exportAllScoresCSV(ds, &buf); err != nil {
		t.Fatalf("exportAllScoresCSV: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (header + 1 row)", len(lines))
	}
	if !strings.HasPrefix(lines[0], "試合日時") {
		t.Errorf("header should start with 試合日時, got %q", lines[0])
	}
}

func TestGetLatestDatetime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scores.csv")

	content := `試合日時,プレイヤーNo.,地域,プレイヤー名,勝利判定,機体名,機体画像URL,スコア,撃墜数,被撃墜数,与ダメージ,被ダメージ,EXダメージ
2025-01-10 18:00,1,東京都,テスト,WIN,ガンダム,img.png,100,2,1,3000,1000,500
2025-01-15 20:30,1,東京都,テスト,LOSE,ザク,img2.png,80,1,2,2000,3000,400
2025-01-12 10:00,1,東京都,テスト,WIN,ガンダム,img.png,120,3,0,4000,500,600
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	latest, err := GetLatestDatetime(path)
	if err != nil {
		t.Fatalf("GetLatestDatetime: %v", err)
	}

	want := time.Date(2025, 1, 15, 20, 30, 0, 0, time.UTC)
	if !latest.Equal(want) {
		t.Errorf("got %v, want %v", latest, want)
	}
}

func TestGetLatestDatetime_NotFound(t *testing.T) {
	latest, err := GetLatestDatetime("/nonexistent/scores.csv")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if !latest.IsZero() {
		t.Errorf("expected zero time, got %v", latest)
	}
}

func TestSaveAllScoresCSV_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.csv")

	ds := model.DatedScores{
		{
			PlayerNo: 1,
			Datetime: time.Date(2025, 1, 15, 20, 30, 0, 0, time.UTC),
			PlayerScore: model.PlayerScore{
				City: "東京都", Name: "テスト", Win: "WIN",
				MsName: "ガンダム", MsImage: "img.png",
				Point: 100, Kills: 2, Deaths: 1,
				Give_damage: 3000, Receive_damage: 1000, Ex_damage: 500,
			},
		},
	}

	if err := SaveAllScoresCSV(ds, path); err != nil {
		t.Fatalf("SaveAllScoresCSV: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2 (header + 1 row)", len(lines))
	}
}

func TestSaveAllScoresCSV_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.csv")

	header := "試合日時,プレイヤーNo.,地域,プレイヤー名,勝利判定,機体名,機体画像URL,スコア,撃墜数,被撃墜数,与ダメージ,被ダメージ,EXダメージ\n"
	header += "2025-01-10 18:00,1,東京都,テスト,WIN,ガンダム,img.png,100,2,1,3000,1000,500\n"
	if err := os.WriteFile(path, []byte(header), 0644); err != nil {
		t.Fatal(err)
	}

	ds := model.DatedScores{
		{
			PlayerNo: 1,
			Datetime: time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			PlayerScore: model.PlayerScore{
				City: "大阪府", Name: "テスト2", Win: "LOSE",
				MsName: "ザク", MsImage: "img2.png",
				Point: 80, Kills: 1, Deaths: 2,
				Give_damage: 2000, Receive_damage: 3000, Ex_damage: 400,
			},
		},
	}

	if err := SaveAllScoresCSV(ds, path); err != nil {
		t.Fatalf("SaveAllScoresCSV append: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 (header + 2 rows)", len(lines))
	}
}

func TestSaveAllScoresCSV_EmptyAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.csv")

	original := "試合日時,header\n2025-01-10 18:00,data\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SaveAllScoresCSV(model.DatedScores{}, path); err != nil {
		t.Fatalf("SaveAllScoresCSV empty: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Error("file should not change when appending empty scores")
	}
}
