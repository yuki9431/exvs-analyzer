package model

import "time"

// MSInfo は機体情報（画像URL → 機体名のマッピング）
type MSInfo struct {
	Name     string
	ImageURL string
	Cost     int `json:",omitempty"`
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
