package main

import (
	"log"
	"os"
	"path/filepath"
)

func main() {
	// 引数チェック: username, password, csvPath の3つが必要
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %s <username> <password> <csv_path>", os.Args[0])
	}

	username := os.Args[1]
	password := os.Args[2]
	csvPath := os.Args[3]

	// ms_list.json をCSVと同じディレクトリに配置
	msListPath := filepath.Join(filepath.Dir(csvPath), "ms_list.json")

	datedScores := Scraiping(username, password)

	// 機体名マッピング: ファイルがあればそこから読む、なければスクレイピングして保存
	msList, err := LoadMSList(msListPath)
	if err != nil {
		log.Println("[INFO] MS list file not found, fetching from ranking page...")
		msList = ScrapeMSList(username, password)
		if err := SaveMSList(msList, msListPath); err != nil {
			log.Printf("[WARN] Failed to save MS list: %v", err)
		} else {
			log.Printf("[INFO] MS list saved to %s (%d entries)", msListPath, len(msList))
		}
	} else {
		log.Printf("[INFO] Loaded MS list from %s (%d entries)", msListPath, len(msList))
	}

	msMap := BuildMSNameMap(msList)
	datedScores.FillMsNames(msMap)

	if err := SaveAllScoresCSV(datedScores, csvPath); err != nil {
		log.Fatalf("[ERROR] Failed to save CSV: %v", err)
	}

	log.Println("[INFO] Scores successfully saved to", csvPath)
}
