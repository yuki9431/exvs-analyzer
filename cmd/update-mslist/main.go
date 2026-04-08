package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/yuki9431/exvs-analyzer/internal/model"
	"github.com/yuki9431/exvs-analyzer/internal/scraper"
)

// imageKey はImageURLからクエリパラメータを除去してキーにする
func imageKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	return u.String()
}

func main() {
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	if username == "" || password == "" {
		log.Fatal("USERNAME and PASSWORD are required")
	}

	outputPath := "data/ms_list.json"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	// 既存のMSリストを読み込み（手動追加分を保持するため）
	existing, err := model.LoadMSList(outputPath)
	if err != nil {
		log.Printf("No existing MS list found, starting fresh: %v", err)
	}

	log.Println("Scraping MS list...")
	scraped := scraper.ScrapeMSList(username, password)

	if len(scraped) == 0 {
		log.Fatal("No MS data found")
	}

	// スクレイピング結果と既存リストをマージ（ImageURLのパス部分で重複排除、クエリパラメータのタイムスタンプ差異を無視）
	seen := make(map[string]bool)
	var merged []model.MSInfo
	for _, ms := range scraped {
		key := imageKey(ms.ImageURL)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, ms)
		}
	}
	for _, ms := range existing {
		key := imageKey(ms.ImageURL)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, ms)
		}
	}

	if err := model.SaveMSList(merged, outputPath); err != nil {
		log.Fatalf("Failed to save MS list: %v", err)
	}

	fmt.Printf("Saved %d MS entries (%d scraped + %d kept from existing) to %s\n",
		len(merged), len(scraped), len(merged)-len(scraped), outputPath)
}
