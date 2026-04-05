package main

import (
	"log"
	"os"
)

const defaultMSListPath = "ms_list.json"

func main() {
	// 引数なし or "serve" → HTTPサーバーモード
	if len(os.Args) == 1 || (len(os.Args) >= 2 && os.Args[1] == "serve") {
		startServer()
		return
	}

	// "update-mslist" → ランキングページから新規機体のみ差分追加
	if len(os.Args) >= 4 && os.Args[1] == "update-mslist" {
		username := os.Args[2]
		password := os.Args[3]
		outputPath := defaultMSListPath
		if len(os.Args) >= 5 {
			outputPath = os.Args[4]
		}

		// 既存リストを読み込み
		existing, err := LoadMSList(outputPath)
		if err != nil {
			existing = []MSInfo{}
		}
		existingURLs := make(map[string]bool, len(existing))
		for _, ms := range existing {
			existingURLs[ms.ImageURL] = true
		}

		// スクレイピングで取得
		log.Println("[INFO] Fetching MS list from ranking page...")
		scraped := ScrapeMSList(username, password)

		// 新規分だけ追加
		added := 0
		for _, ms := range scraped {
			if !existingURLs[ms.ImageURL] {
				existing = append(existing, ms)
				existingURLs[ms.ImageURL] = true
				added++
				log.Printf("[INFO] New MS added: %s", ms.Name)
			}
		}

		if err := SaveMSList(existing, outputPath); err != nil {
			log.Fatalf("[ERROR] Failed to save MS list: %v", err)
		}
		log.Printf("[INFO] MS list updated: %d new, %d total", added, len(existing))
		return
	}

	// CLIモード: username, password, csvPath の3つが必要
	if len(os.Args) < 4 {
		log.Fatalf("Usage:\n  %s <username> <password> <csv_path>\n  %s serve\n  %s update-mslist <username> <password> [output_path]",
			os.Args[0], os.Args[0], os.Args[0])
	}

	username := os.Args[1]
	password := os.Args[2]
	csvPath := os.Args[3]

	// 既存CSVの最新日時を取得し、それ以降の戦歴のみスクレイピング
	since, err := getLatestDatetime(csvPath)
	if err != nil {
		log.Printf("[WARN] Failed to read existing CSV: %v", err)
	}
	if !since.IsZero() {
		log.Printf("[INFO] Fetching scores after %s", since.Format("2006-01-02 15:04"))
	}

	datedScores := Scraiping(username, password, since)

	// 同梱のMSリストから機体名マッピングを読み込み
	msList, err := LoadMSList(defaultMSListPath)
	if err != nil {
		log.Printf("[WARN] MS list not found, MS names will be empty")
	} else {
		log.Printf("[INFO] Loaded MS list (%d entries)", len(msList))
	}

	msMap := BuildMSNameMap(msList)
	datedScores.FillMsNames(msMap)
	datedScores.CheckUnknownMS()

	if err := SaveAllScoresCSV(datedScores, csvPath); err != nil {
		log.Fatalf("[ERROR] Failed to save CSV: %v", err)
	}

	log.Println("[INFO] Scores successfully saved to", csvPath)
}
