package storage

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

// currentCSVColumnCount は現在のCSVフォーマットのカラム数
const currentCSVColumnCount = 22

// NeedsBackfill は既存CSVにバックフィルが必要かどうかを判定する。
// ヘッダーが旧フォーマット、または直近30日以内のデータに新フィールドが空のレコードがあればtrue。
func NeedsBackfill(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return false
	}

	// ヘッダーが旧フォーマットならバックフィル必要
	if len(records[0]) < currentCSVColumnCount {
		return true
	}

	// 直近30日以内のデータで新フィールド（ランク: カラム13）が空のレコードがあるか
	layout := "2006-01-02 15:04"
	cutoff := time.Now().AddDate(0, 0, -30)
	for i, row := range records {
		if i == 0 {
			continue
		}
		dt, err := time.Parse(layout, row[0])
		if err != nil {
			continue
		}
		// 直近30日以内で、カラム数不足または新フィールドが空
		if dt.After(cutoff) && (len(row) < currentCSVColumnCount || row[13] == "") {
			return true
		}
	}
	return false
}

// BackfillDates は直近30日以内で新フィールドが空のレコードの日付セットを返す。
// 日付は "2006/01/02" 形式（スクレイパーのdailyLink.dateと同じ形式）。
func BackfillDates(path string) map[string]bool {
	dates := make(map[string]bool)

	f, err := os.Open(path)
	if err != nil {
		return dates
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return dates
	}

	layout := "2006-01-02 15:04"
	cutoff := time.Now().AddDate(0, 0, -30)
	for i, row := range records {
		if i == 0 {
			continue
		}
		dt, err := time.Parse(layout, row[0])
		if err != nil {
			continue
		}
		if dt.After(cutoff) && (len(row) < currentCSVColumnCount || row[13] == "") {
			dates[dt.Format("2006/01/02")] = true
		}
	}
	return dates
}

// GetLatestDatetime は既存CSVファイルから最新の試合日時を取得する。
// ファイルが存在しない場合はゼロ値のtimeを返す。
func GetLatestDatetime(path string) (time.Time, error) {
	var latest time.Time
	layout := "2006-01-02 15:04"

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return latest, nil
		}
		return latest, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return latest, err
	}

	for i, row := range records {
		if i == 0 || len(row) == 0 {
			continue
		}
		t, err := time.Parse(layout, row[0])
		if err != nil {
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}

	return latest, nil
}

func exportAllScoresCSV(ds model.DatedScores, w io.Writer) error {
	csvw := csv.NewWriter(w)
	defer csvw.Flush()

	header := []string{"試合日時", "プレイヤーNo.", "地域", "プレイヤー名", "勝利判定", "機体名", "機体画像URL", "スコア", "撃墜数", "被撃墜数", "与ダメージ", "被ダメージ", "EXダメージ", "ランク", "チーム名", "称号画像URL", "称号バッジURL", "プロフィールURL", "シャッフルグレード画像URL", "チームグレード画像URL", "順位バッジURL", "店舗名"}
	if err := csvw.Write(header); err != nil {
		return err
	}

	for _, d := range ds {
		row := scoreToRow(d)
		if err := csvw.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func scoreToRow(d model.DatedScore) []string {
	return []string{
		d.Datetime.Format("2006-01-02 15:04"),
		strconv.Itoa(d.PlayerNo),
		d.PlayerScore.City,
		d.PlayerScore.Name,
		d.PlayerScore.Win,
		d.PlayerScore.MsName,
		d.PlayerScore.MsImage,
		strconv.Itoa(d.PlayerScore.Point),
		strconv.Itoa(d.PlayerScore.Kills),
		strconv.Itoa(d.PlayerScore.Deaths),
		strconv.Itoa(d.PlayerScore.Give_damage),
		strconv.Itoa(d.PlayerScore.Receive_damage),
		strconv.Itoa(d.PlayerScore.Ex_damage),
		d.PlayerScore.Mastery,
		d.PlayerScore.TeamName,
		d.PlayerScore.TitleImage,
		d.PlayerScore.TitleBadge,
		d.PlayerScore.ProfileLink,
		d.PlayerScore.ShuffleGrade,
		d.PlayerScore.TeamGrade,
		d.PlayerScore.RankingImage,
		d.PlayerScore.ShopName,
	}
}

// ReadAllScoresCSV は既存CSVから全レコードを読み込む。
// 旧フォーマット（カラム数不足）のデータも新フィールドを空値として読み込む。
func ReadAllScoresCSV(path string) (model.DatedScores, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // カラム数不一致を許容
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	layout := "2006-01-02 15:04"
	var scores model.DatedScores

	for i, row := range records {
		if i == 0 || len(row) < 13 {
			continue
		}
		dt, err := time.Parse(layout, row[0])
		if err != nil {
			continue
		}
		playerNo, _ := strconv.Atoi(row[1])
		point, _ := strconv.Atoi(row[7])
		kills, _ := strconv.Atoi(row[8])
		deaths, _ := strconv.Atoi(row[9])
		giveDamage, _ := strconv.Atoi(row[10])
		receiveDamage, _ := strconv.Atoi(row[11])
		exDamage, _ := strconv.Atoi(row[12])

		col := func(idx int) string {
			if idx < len(row) {
				return row[idx]
			}
			return ""
		}

		scores = append(scores, model.DatedScore{
			PlayerNo: playerNo,
			Datetime: dt,
			PlayerScore: model.PlayerScore{
				City:           row[2],
				Name:           row[3],
				Win:            row[4],
				MsName:         row[5],
				MsImage:        row[6],
				Point:          point,
				Kills:          kills,
				Deaths:         deaths,
				Give_damage:    giveDamage,
				Receive_damage: receiveDamage,
				Ex_damage:      exDamage,
				Mastery:        col(13),
				TeamName:       col(14),
				TitleImage:     col(15),
				TitleBadge:     col(16),
				ProfileLink:    col(17),
				ShuffleGrade:   col(18),
				TeamGrade:      col(19),
				RankingImage:   col(20),
				ShopName:       col(21),
			},
		})
	}

	return scores, nil
}

// SaveAllScoresCSV は既存CSVに新しいレコードのみ追記する。
// ファイルが存在しない場合は新規作成する。
func SaveAllScoresCSV(ds model.DatedScores, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return exportAllScoresCSV(ds, f)
	}

	if len(ds) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	csvw := csv.NewWriter(f)
	defer csvw.Flush()

	for _, d := range ds {
		row := scoreToRow(d)
		if err := csvw.Write(row); err != nil {
			return err
		}
	}

	return nil
}
