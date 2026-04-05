// scraper_test.go — internal/scraper パッケージのユニットテスト
//
// テスト対象と観点:
//   - parseNumber: カンマ区切りの数値文字列→int変換。空文字や非数値も確認
//   - dateFormatDaily: 日単位に切り捨て（時刻をゼロに、JSTタイムゾーン）
//   - dateFormatMonthly: 月単位に切り捨て（日を1日に、JSTタイムゾーン）
//   - GetDailyScores: 指定日のスコアだけ抽出されるか
//   - GetMonthlyScores: 指定月のスコアだけ抽出されるか
//   - GetAverage: スコア平均の計算。勝利数カウントも確認
//   - GetDateList: 日付一覧の重複排除。daily/monthly/不正引数
//
// 外部サイトに接続する Scraping/ScrapeMSList はテスト対象外。
//
// 実行方法:
//   make test
package scraper

import (
	"testing"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,234", 1234},
		{"100", 100},
		{"0", 0},
		{"12,345,678", 12345678},
		{"", 0},
		{"abc", 0},
		{"score: 1,500pt", 1500},
	}

	for _, tt := range tests {
		got := parseNumber(tt.input)
		if got != tt.want {
			t.Errorf("parseNumber(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDateFormatDaily(t *testing.T) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	input := time.Date(2025, 3, 15, 20, 30, 45, 0, time.UTC)
	got := dateFormatDaily(input)

	want := time.Date(2025, 3, 15, 0, 0, 0, 0, jst)
	if !got.Equal(want) {
		t.Errorf("dateFormatDaily = %v, want %v", got, want)
	}
}

func TestDateFormatMonthly(t *testing.T) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	input := time.Date(2025, 3, 15, 20, 30, 45, 0, time.UTC)
	got := dateFormatMonthly(input)

	want := time.Date(2025, 3, 1, 0, 0, 0, 0, jst)
	if !got.Equal(want) {
		t.Errorf("dateFormatMonthly = %v, want %v", got, want)
	}
}

func newDatedScore(year int, month time.Month, day, hour int, name, win string, point int) model.DatedScore {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	return model.DatedScore{
		PlayerNo: 1,
		Datetime: time.Date(year, month, day, hour, 0, 0, 0, jst),
		PlayerScore: model.PlayerScore{
			Name:  name,
			Win:   win,
			Point: point,
		},
	}
}

func TestGetDailyScores(t *testing.T) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	ds := model.DatedScores{
		newDatedScore(2025, 3, 15, 10, "A", "win", 100),
		newDatedScore(2025, 3, 15, 20, "B", "lose", 80),
		newDatedScore(2025, 3, 16, 10, "C", "win", 120),
	}

	target := time.Date(2025, 3, 15, 0, 0, 0, 0, jst)
	got := GetDailyScores(ds, target)

	if len(got) != 2 {
		t.Fatalf("got %d scores, want 2", len(got))
	}
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Errorf("got names %q, %q, want A, B", got[0].Name, got[1].Name)
	}
}

func TestGetMonthlyScores(t *testing.T) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	ds := model.DatedScores{
		newDatedScore(2025, 3, 15, 10, "A", "win", 100),
		newDatedScore(2025, 3, 28, 20, "B", "lose", 80),
		newDatedScore(2025, 4, 1, 10, "C", "win", 120),
	}

	target := time.Date(2025, 3, 1, 0, 0, 0, 0, jst)
	got := GetMonthlyScores(ds, target)

	if len(got) != 2 {
		t.Fatalf("got %d scores, want 2", len(got))
	}
}

func TestGetAverage(t *testing.T) {
	scores := model.PlayerScores{
		{Win: "win", Point: 100, Kills: 3, Deaths: 1, Give_damage: 5000, Receive_damage: 2000, Ex_damage: 1000},
		{Win: "lose", Point: 80, Kills: 1, Deaths: 2, Give_damage: 3000, Receive_damage: 4000, Ex_damage: 500},
	}

	avg := GetAverage(scores)

	if avg.Game_count != 2 {
		t.Errorf("Game_count = %d, want 2", avg.Game_count)
	}
	if avg.Victories != 1 {
		t.Errorf("Victories = %d, want 1", avg.Victories)
	}
	if avg.PlayerScore.Point != 90 {
		t.Errorf("Point avg = %d, want 90", avg.PlayerScore.Point)
	}
	if avg.PlayerScore.Kills != 2 {
		t.Errorf("Kills avg = %d, want 2", avg.PlayerScore.Kills)
	}
}

func TestGetDateList_Daily(t *testing.T) {
	ds := model.DatedScores{
		newDatedScore(2025, 3, 15, 10, "A", "win", 100),
		newDatedScore(2025, 3, 15, 20, "B", "lose", 80),
		newDatedScore(2025, 3, 16, 10, "C", "win", 120),
	}

	dates, err := GetDateList(ds, "daily")
	if err != nil {
		t.Fatalf("GetDateList: %v", err)
	}
	if len(dates) != 2 {
		t.Errorf("got %d dates, want 2 (same day deduplicated)", len(dates))
	}
}

func TestGetDateList_Monthly(t *testing.T) {
	ds := model.DatedScores{
		newDatedScore(2025, 3, 15, 10, "A", "win", 100),
		newDatedScore(2025, 3, 28, 20, "B", "lose", 80),
		newDatedScore(2025, 4, 1, 10, "C", "win", 120),
	}

	dates, err := GetDateList(ds, "monthly")
	if err != nil {
		t.Fatalf("GetDateList: %v", err)
	}
	if len(dates) != 2 {
		t.Errorf("got %d dates, want 2 (same month deduplicated)", len(dates))
	}
}

func TestGetDateList_InvalidFrequency(t *testing.T) {
	ds := model.DatedScores{
		newDatedScore(2025, 3, 15, 10, "A", "win", 100),
	}

	_, err := GetDateList(ds, "weekly")
	if err == nil {
		t.Error("expected error for invalid frequency, got nil")
	}
}
