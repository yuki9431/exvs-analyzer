// scraper_test.go — internal/scraper パッケージのユニットテスト
//
// テスト対象と観点:
//   - parseNumber: カンマ区切りの数値文字列→int変換。空文字や非数値も確認
//
// 外部サイトに接続する Scraping/ScrapeMSList はテスト対象外。
//
// 実行方法:
//
//	make test
package scraper

import (
	"testing"
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
