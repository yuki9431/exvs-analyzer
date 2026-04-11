package mslist

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"sort"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

// stripQuery はURLからクエリパラメータを除去する
func stripQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	return u.String()
}

// BuildMSNameMap はMSInfoリストから画像URL→機体名のマップを生成する（クエリパラメータを除去してマッチ）
func BuildMSNameMap(msList []model.MSInfo) map[string]string {
	m := make(map[string]string, len(msList))
	for _, ms := range msList {
		m[stripQuery(ms.ImageURL)] = ms.Name
	}
	return m
}

// FillMsNames はDatedScoresの各スコアにMsNameをセットする（クエリパラメータを除去してマッチ）
func FillMsNames(ds model.DatedScores, msMap map[string]string) {
	for i := range ds {
		if name, ok := msMap[stripQuery(ds[i].PlayerScore.MsImage)]; ok {
			ds[i].PlayerScore.MsName = name
		}
	}
}

// CheckUnknownMS はMsNameが空のままの機体画像URLをログに出力する
func CheckUnknownMS(ds model.DatedScores) {
	unknown := make(map[string]int)
	for _, d := range ds {
		if d.PlayerScore.MsImage != "" && d.PlayerScore.MsName == "" {
			unknown[d.PlayerScore.MsImage]++
		}
	}
	for url, count := range unknown {
		log.Printf("[ALERT] Unknown MS (appeared %d times): %s", count, url)
	}
	if len(unknown) > 0 {
		log.Printf("[ALERT] %d unknown MS found. Run 'update-mslist' or add them to ms_list.json manually.", len(unknown))
	}
}

// MergeMSList はスクレイピング結果と既存リストをマージする（ImageURLのパス部分で重複排除、クエリパラメータを除去）
// 既存データの色違い機体（同名・別URL）を保持し、コスト情報を名前ベースで補完する
func MergeMSList(scraped, existing []model.MSInfo) []model.MSInfo {
	// スクレイピング結果から名前→コストのマップを作成
	costByName := make(map[string]int)
	for _, ms := range scraped {
		if ms.Cost > 0 {
			costByName[ms.Name] = ms.Cost
		}
	}

	seen := make(map[string]bool)
	var merged []model.MSInfo
	for _, ms := range scraped {
		key := stripQuery(ms.ImageURL)
		if !seen[key] {
			seen[key] = true
			ms.ImageURL = key
			merged = append(merged, ms)
		}
	}
	for _, ms := range existing {
		key := stripQuery(ms.ImageURL)
		if !seen[key] {
			seen[key] = true
			ms.ImageURL = key
			// 既存データにコストがなければ名前ベースで補完
			if ms.Cost == 0 {
				if cost, ok := costByName[ms.Name]; ok {
					ms.Cost = cost
				}
			}
			merged = append(merged, ms)
		}
	}
	return merged
}

// SaveMSList はMSInfoリストをName→ImageURLの順でソートしてJSONファイルに保存する
func SaveMSList(msList []model.MSInfo, path string) error {
	sort.Slice(msList, func(i, j int) bool {
		if msList[i].Name != msList[j].Name {
			return msList[i].Name < msList[j].Name
		}
		return msList[i].ImageURL < msList[j].ImageURL
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(msList)
}

// LoadMSList はJSONファイルからMSInfoリストを読み込む
func LoadMSList(path string) ([]model.MSInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var msList []model.MSInfo
	if err := json.NewDecoder(f).Decode(&msList); err != nil {
		return nil, err
	}
	return msList, nil
}
