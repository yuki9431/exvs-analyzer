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
	Mastery        string // ランク(master, gold2, silver5等)
	TeamName       string // チーム名
	TitleImage     string // 称号画像URL
	TitleBadge     string // 称号バッジURL
	ProfileLink    string // プロフィールページURL
	ShuffleGrade   string // シャッフル階級画像URL
	TeamGrade      string // チーム(固定)階級画像URL
	RankingImage   string // スコア順位バッジ画像URL
	ShopName       string // プレイ店舗名
}

// MatchEvent は試合経過の1イベント
type MatchEvent struct {
	Group    string  `json:"group"`     // team1-1, team1-2, team2-1, team2-2
	StartSec float64 `json:"start_sec"` // 開始時間(秒)
	EndSec   float64 `json:"end_sec"`   // 終了時間(秒、pointの場合は0)
	ClassName string `json:"class_name"` // ex, exbst-f, exbst-s, exbst-e, ov, exbst-ov, xb
	IsPoint  bool    `json:"is_point"`  // 被撃墜イベントか
}

// MatchTimeline は試合全体の経過データ
type MatchTimeline struct {
	Events     []MatchEvent `json:"events"`
	GameEndSec float64      `json:"game_end_sec"`
}

// DatedScore は日付付きスコア
type DatedScore struct {
	PlayerNo      int
	Datetime       time.Time
	PlayerScore    PlayerScore
	MatchTimeline *MatchTimeline // 試合経過(PlayerNo==1のときのみセット、4人で共有)
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
