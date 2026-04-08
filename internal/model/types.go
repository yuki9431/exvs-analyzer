package model

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"sort"
	"time"
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

// MSInfo は機体情報（画像URL → 機体名のマッピング）
type MSInfo struct {
	Name     string
	ImageURL string
}

// PlayerScore はスコア
type PlayerScore struct {
	City           string
	Name           string
	Win            string
	MsImage        string
	MsName         string
	Point          int
	Kills          int
	Deaths         int
	Give_damage    int
	Receive_damage int
	Ex_damage      int
}

// DatedScore は日付付きスコア
type DatedScore struct {
	PlayerNo    int
	Datetime    time.Time
	PlayerScore PlayerScore
}

// AverageScore はスコア平均
type AverageScore struct {
	Game_count  int
	Victories   int
	PlayerScore PlayerScore
}

// PlayerScores はスコアのリスト
type PlayerScores []PlayerScore

// DatedScores は日付付きスコアのリスト
type DatedScores []DatedScore

// BuildMSNameMap はMSInfoリストから画像URL→機体名のマップを生成する（クエリパラメータを除去してマッチ）
func BuildMSNameMap(msList []MSInfo) map[string]string {
	m := make(map[string]string, len(msList))
	for _, ms := range msList {
		m[stripQuery(ms.ImageURL)] = ms.Name
	}
	return m
}

// FillMsNames はDatedScoresの各スコアにMsNameをセットする（クエリパラメータを除去してマッチ）
func (ds DatedScores) FillMsNames(msMap map[string]string) {
	for i := range ds {
		if name, ok := msMap[stripQuery(ds[i].PlayerScore.MsImage)]; ok {
			ds[i].PlayerScore.MsName = name
		}
	}
}

// CheckUnknownMS はMsNameが空のままの機体画像URLをログに出力する
func (ds DatedScores) CheckUnknownMS() {
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

// SaveMSList はMSInfoリストを名前順でソートしてJSONファイルに保存する
func SaveMSList(msList []MSInfo, path string) error {
	sort.Slice(msList, func(i, j int) bool {
		return msList[i].Name < msList[j].Name
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
func LoadMSList(path string) ([]MSInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var msList []MSInfo
	if err := json.NewDecoder(f).Decode(&msList); err != nil {
		return nil, err
	}
	return msList, nil
}
