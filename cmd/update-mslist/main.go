package main

import (
	"fmt"
	"log"
	"os"

	"github.com/yuki9431/exvs-analyzer/internal/model"
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

	log.Println("Scraping MS list...")
	msList := scraper.ScrapeMSList(username, password)

	if len(msList) == 0 {
		log.Fatal("No MS data found")
	}

	if err := model.SaveMSList(msList, outputPath); err != nil {
		log.Fatalf("Failed to save MS list: %v", err)
	}

	fmt.Printf("Saved %d MS entries to %s\n", len(msList), outputPath)
}
