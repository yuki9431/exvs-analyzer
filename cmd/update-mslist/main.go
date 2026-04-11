package main

import (
	"fmt"
	"log"
	"os"

	"github.com/yuki9431/exvs-analyzer/internal/mslist"
	"github.com/yuki9431/exvs-analyzer/internal/scraper"
)

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

	existing, err := mslist.LoadMSList(outputPath)
	if err != nil {
		log.Printf("No existing MS list found, starting fresh: %v", err)
	}

	log.Println("Scraping MS list...")
	scraped := scraper.ScrapeMSList(username, password)

	if len(scraped) == 0 {
		log.Fatal("No MS data found")
	}

	merged := mslist.MergeMSList(scraped, existing)

	if err := mslist.SaveMSList(merged, outputPath); err != nil {
		log.Fatalf("Failed to save MS list: %v", err)
	}

	fmt.Printf("Saved %d MS entries (%d scraped + %d kept from existing) to %s\n",
		len(merged), len(scraped), len(merged)-len(scraped), outputPath)
}
