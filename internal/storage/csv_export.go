package storage

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
)

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

	header := []string{"試合日時", "プレイヤーNo.", "地域", "プレイヤー名", "勝利判定", "機体名", "機体画像URL", "スコア", "撃墜数", "被撃墜数", "与ダメージ", "被ダメージ", "EXダメージ"}
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
	}
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
