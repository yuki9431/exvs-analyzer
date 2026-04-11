// mslist_test.go — internal/mslist パッケージのユニットテスト
//
// テスト対象と観点:
//   - BuildMSNameMap: MSリスト→マップ変換。空リスト・存在しないキーも確認
//   - FillMsNames: マップを使ったMS名補完。マッチしないURLは空のまま残るか
//   - SaveMSList/LoadMSList: JSON保存→読み込みの往復。名前順ソートの確認
//   - CheckUnknownMS: 不明MSがあってもパニックしないか（ログ出力のみ）
//
// テストデータはすべて架空の値。実際のms_list.jsonとは無関係。
// ファイル系テストは t.TempDir() で一時フォルダを使い、実データに影響しない。
//
// 実行方法:
//
//	make test
package mslist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

func TestBuildMSNameMap(t *testing.T) {
	msList := []model.MSInfo{
		{Name: "ガンダム", ImageURL: "https://example.com/gundam.png"},
		{Name: "ザク", ImageURL: "https://example.com/zaku.png"},
	}

	m := BuildMSNameMap(msList)

	if got := m["https://example.com/gundam.png"]; got != "ガンダム" {
		t.Errorf("got %q, want %q", got, "ガンダム")
	}
	if got := m["https://example.com/zaku.png"]; got != "ザク" {
		t.Errorf("got %q, want %q", got, "ザク")
	}
	if got := m["https://example.com/unknown.png"]; got != "" {
		t.Errorf("got %q for unknown key, want empty", got)
	}
}

func TestBuildMSNameMap_Empty(t *testing.T) {
	m := BuildMSNameMap(nil)
	if len(m) != 0 {
		t.Errorf("got len %d, want 0", len(m))
	}
}

func TestFillMsNames(t *testing.T) {
	msMap := map[string]string{
		"https://example.com/gundam.png": "ガンダム",
	}
	ds := model.DatedScores{
		{PlayerNo: 1, PlayerScore: model.PlayerScore{MsImage: "https://example.com/gundam.png"}},
		{PlayerNo: 2, PlayerScore: model.PlayerScore{MsImage: "https://example.com/unknown.png"}},
	}

	FillMsNames(ds, msMap)

	if got := ds[0].PlayerScore.MsName; got != "ガンダム" {
		t.Errorf("player 1: got %q, want %q", got, "ガンダム")
	}
	if got := ds[1].PlayerScore.MsName; got != "" {
		t.Errorf("player 2: got %q, want empty (unknown MS)", got)
	}
}

func TestFillMsNames_EmptyMap(t *testing.T) {
	ds := model.DatedScores{
		{PlayerNo: 1, Datetime: time.Now(), PlayerScore: model.PlayerScore{MsImage: "https://example.com/gundam.png"}},
	}
	FillMsNames(ds, map[string]string{})

	if got := ds[0].PlayerScore.MsName; got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSaveAndLoadMSList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ms_list.json")

	original := []model.MSInfo{
		{Name: "ザク", ImageURL: "https://example.com/zaku.png"},
		{Name: "ガンダム", ImageURL: "https://example.com/gundam.png"},
	}

	if err := SaveMSList(original, path); err != nil {
		t.Fatalf("SaveMSList: %v", err)
	}

	loaded, err := LoadMSList(path)
	if err != nil {
		t.Fatalf("LoadMSList: %v", err)
	}

	// SaveMSListは名前順ソートするので、ガンダムが先
	if loaded[0].Name != "ガンダム" {
		t.Errorf("got %q at index 0, want %q (should be sorted)", loaded[0].Name, "ガンダム")
	}
	if loaded[1].Name != "ザク" {
		t.Errorf("got %q at index 1, want %q", loaded[1].Name, "ザク")
	}
	if len(loaded) != 2 {
		t.Errorf("got %d items, want 2", len(loaded))
	}
}

func TestLoadMSList_NotFound(t *testing.T) {
	_, err := LoadMSList("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestSaveMSList_InvalidPath(t *testing.T) {
	err := SaveMSList([]model.MSInfo{{Name: "test"}}, "/nonexistent/dir/file.json")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestSaveMSList_Sort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sorted.json")

	msList := []model.MSInfo{
		{Name: "C", ImageURL: "c"},
		{Name: "A", ImageURL: "a"},
		{Name: "B", ImageURL: "b"},
	}

	if err := SaveMSList(msList, path); err != nil {
		t.Fatalf("SaveMSList: %v", err)
	}

	if msList[0].Name != "A" || msList[1].Name != "B" || msList[2].Name != "C" {
		t.Errorf("original slice not sorted: %v", msList)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("saved file not found: %v", err)
	}
}

func TestCheckUnknownMS(t *testing.T) {
	// CheckUnknownMSはログ出力のみなので、パニックしないことを確認
	ds := model.DatedScores{
		{PlayerScore: model.PlayerScore{MsImage: "https://example.com/unknown.png", MsName: ""}},
		{PlayerScore: model.PlayerScore{MsImage: "https://example.com/gundam.png", MsName: "ガンダム"}},
		{PlayerScore: model.PlayerScore{MsImage: "", MsName: ""}},
	}
	CheckUnknownMS(ds)
}
