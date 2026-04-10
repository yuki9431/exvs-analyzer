package scraper

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/yuki9431/exvs-analyzer/internal/model"
)

const (
	vsmobile          = "web.vsmobile.jp"
	mobileRankpage    = "https://web.vsmobile.jp/exvs2ib/results/classmatch/fight"
	mobileMSUsedRate  = "https://web.vsmobile.jp/exvs2ib/ranking/ms_used_rate"
)

func parseNumber(s string) int {
	re := regexp.MustCompile(`[\d,]+`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	m = strings.ReplaceAll(m, ",", "")
	v, _ := strconv.Atoi(m)
	return v
}

func dateFormatDaily(t time.Time) time.Time {
	var jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst)
}

func dateFormatMonthly(t time.Time) time.Time {
	var jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, jst)
}

// ProgressFunc はスクレイピングの進捗を通知するコールバック型
type ProgressFunc func(message string)

// Scraping はスクレイピング処理を実行し、DatedScoresを返す
func Scraping(username, password string, since time.Time, onProgress ...ProgressFunc) model.DatedScores {
	var (
		scores     model.DatedScores
		date, hour string
		wins       []string
	)

	notify := func(msg string) {
		if len(onProgress) > 0 && onProgress[0] != nil {
			onProgress[0](msg)
		}
	}

	m := NewClient(username, password)
	notify("ログイン中...")
	m.Login()

	rankpage := colly.NewCollector(
		colly.AllowedDomains(vsmobile),
	)
	rankpage.SetCookieJar(m.HTTPClient.Jar)
	dailypage := rankpage.Clone()
	detailpage := rankpage.Clone()

	rankpage.OnHTML("li.item", func(e *colly.HTMLElement) {
		r := regexp.MustCompile(`\(.*`)
		date = r.ReplaceAllString(e.ChildText("p.datetime.fz-ss"), "")

		if !since.IsZero() {
			d, err := time.Parse("2006/01/02", date)
			if err == nil && d.Before(since.Truncate(24*time.Hour)) {
				return
			}
		}

		link := e.ChildAttr("a", "href")
		dailypage.Visit(link)
	})

	dailypage.OnHTML("li.item", func(e *colly.HTMLElement) {
		hour = e.ChildText("p.datetime.fz-ss")

		if !since.IsZero() {
			t, err := time.Parse("2006/01/02 15:04", date+" "+hour)
			if err == nil && !t.After(since) {
				return
			}
		}

		if d, err := time.Parse("2006/01/02 15:04", date+" "+hour); err == nil {
			notify(fmt.Sprintf("%sの戦歴データを取得中...", d.Format("01/02 15:04")))
		}

		if e.ChildAttr("a", "class") == "right-arrow vs-detail win" {
			wins = []string{"win", "win", "lose", "lose"}
		} else {
			wins = []string{"lose", "lose", "win", "win"}
		}

		link := e.ChildAttr("a", "href")
		detailpage.Visit(link)
	})

	dailypage.OnHTML("div.block.control", func(e *colly.HTMLElement) {
		// 「>」(次へ)ボタンは末尾から2番目のリンク
		links := e.ChildAttrs("ul.clearfix > li > a", "href")
		if len(links) >= 2 {
			nextLink := links[len(links)-2]
			if nextLink != "javascript:void(0);" {
				dailypage.Visit(nextLink)
			}
		}
	})

	detailpage.OnHTML("div.panel_area", func(e *colly.HTMLElement) {
		selectorLeftValue := "div.w45.pr-ss > dl > dd"
		selectorRightValue := "div.w55 > dl > dd"
		selectorCity := "div.w80.ta-r > p.col-stand"
		selectorName := "p.mb-ss.fz-m > span.name"
		selectorMSImage := "#panel3 img.item-icon-img"

		cities := e.ChildTexts(selectorCity)
		names := e.ChildTexts(selectorName)
		msImages := e.ChildAttrs(selectorMSImage, "data-original")
		leftValue := e.ChildTexts(selectorLeftValue)
		rightValue := e.ChildTexts(selectorRightValue)

		var layout = "2006/01/02 15:04"
		t := date + " " + hour
		datatime, _ := time.Parse(layout, t)

		playerCount := 4

		for i := 0; i < playerCount; i++ {
			offL := i * 3
			offR := i * 3

			city := cities[i]
			name := names[i]
			win := wins[i]
			msImage := ""
			if i < len(msImages) {
				msImage = msImages[i]
			}
			point := parseNumber(leftValue[0+offL])
			kills := parseNumber(leftValue[1+offL])
			deaths := parseNumber(leftValue[2+offL])
			giveDamage := parseNumber(rightValue[0+offR])
			receiveDamage := parseNumber(rightValue[1+offR])
			exDamage := parseNumber(rightValue[2+offR])

			result := model.DatedScore{
				PlayerNo: i + 1,
				Datetime: datatime,
				PlayerScore: model.PlayerScore{
					City:           city,
					Name:           name,
					Win:            win,
					MsImage:        msImage,
					MsName:         "",
					Point:          point,
					Kills:          kills,
					Deaths:         deaths,
					Give_damage:    giveDamage,
					Receive_damage: receiveDamage,
					Ex_damage:      exDamage,
				},
			}

			scores = append(scores, result)
		}
	})

	rankpage.Visit(mobileRankpage)
	return scores
}

// ScrapeMSList は機体使用率ランキングページから画像URLと機体名の一覧を取得する
func ScrapeMSList(username, password string) []model.MSInfo {
	var msList []model.MSInfo
	seen := make(map[string]bool)

	m := NewClient(username, password)
	m.Login()

	// まずCSRFトークンを取得
	var csrfToken string
	tokenCollector := colly.NewCollector(colly.AllowedDomains(vsmobile))
	tokenCollector.SetCookieJar(m.HTTPClient.Jar)
	tokenCollector.OnHTML("input[name=_token]", func(e *colly.HTMLElement) {
		csrfToken = e.Attr("value")
	})
	tokenCollector.Visit(mobileMSUsedRate)

	// 各コストでPOSTしてMS一覧を取得
	costs := []int{3000, 2500, 2000, 1500}
	for _, cost := range costs {
		currentCost := cost

		c := colly.NewCollector(
			colly.AllowedDomains(vsmobile),
		)
		c.SetCookieJar(m.HTTPClient.Jar)

		c.OnHTML("li.item div.ds-fx.fx-va-s.fx-hz-s", func(e *colly.HTMLElement) {
			imageURL := e.ChildAttr("img.item-icon-img", "data-original")
			name := strings.TrimSpace(e.ChildText("div.prompt-area > p.fz-s"))

			if imageURL != "" && name != "" && !seen[imageURL] {
				seen[imageURL] = true
				msList = append(msList, model.MSInfo{
					Name:     name,
					ImageURL: imageURL,
					Cost:     currentCost,
				})
			}
		})

		c.OnHTML("div.page-send ul.clearfix", func(e *colly.HTMLElement) {
			nextLinks := e.ChildAttrs("li > a", "href")
			for _, link := range nextLinks {
				if link != "javascript:void(0);" {
					c.Visit(e.Request.AbsoluteURL(link))
				}
			}
		})

		c.Post(mobileMSUsedRate, map[string]string{
			"_token":   csrfToken,
			"cost":     fmt.Sprintf("%d", currentCost),
			"category": "1",
		})
	}

	return msList
}

// GetDailyScores は指定した日のスコアを取得する
func GetDailyScores(ds model.DatedScores, t time.Time) model.PlayerScores {
	return getScores(ds, t, dateFormatDaily)
}

// GetMonthlyScores は指定した月のスコアを取得する
func GetMonthlyScores(ds model.DatedScores, t time.Time) model.PlayerScores {
	return getScores(ds, t, dateFormatMonthly)
}

func getScores(ds model.DatedScores, t time.Time, format func(time.Time) time.Time) model.PlayerScores {
	var scores model.PlayerScores
	date := format(t)
	for _, v := range ds {
		vd := format(v.Datetime)
		if vd.Equal(date) {
			scores = append(scores, v.PlayerScore)
		}
	}
	return scores
}

// GetAverage はスコアリストの値を合計しAverageScoreを取得する
func GetAverage(s model.PlayerScores) model.AverageScore {
	var (
		gameCount      = 0
		sumVictories   = 0
		sumPoint       = 0
		sumKills       = 0
		sumDeaths      = 0
		sumGiveDamage  = 0
		sumReceiveDmg  = 0
		sumExDamage    = 0
	)

	for _, v := range s {
		sumPoint += v.Point
		sumKills += v.Kills
		sumDeaths += v.Deaths
		sumGiveDamage += v.Give_damage
		sumReceiveDmg += v.Receive_damage
		sumExDamage += v.Ex_damage
		gameCount++
		if v.Win == "win" {
			sumVictories++
		}
	}

	n := float64(len(s))
	return model.AverageScore{
		Game_count: gameCount,
		Victories:  sumVictories,
		PlayerScore: model.PlayerScore{
			Point:          int(math.Round(float64(sumPoint) / n)),
			Kills:          int(math.Round(float64(sumKills) / n)),
			Deaths:         int(math.Round(float64(sumDeaths) / n)),
			Give_damage:    int(math.Round(float64(sumGiveDamage) / n)),
			Receive_damage: int(math.Round(float64(sumReceiveDmg) / n)),
			Ex_damage:      int(math.Round(float64(sumExDamage) / n)),
		},
	}
}

// GetDateList は対戦を行った日付一覧を取得する
func GetDateList(ds model.DatedScores, frequency string) ([]time.Time, error) {
	var dates []time.Time
	var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

	for _, v := range ds {
		var day int
		switch frequency {
		case "daily":
			day = v.Datetime.Day()
		case "monthly":
			day = 1
		default:
			return nil, errors.New(`ERROR: "daily" or "monthly" is required for the argument`)
		}
		d := time.Date(v.Datetime.Year(), v.Datetime.Month(), day, 0, 0, 0, 0, jst)
		dates = append(dates, d)
	}

	var datelist []time.Time
	m := make(map[time.Time]bool)
	for _, v := range dates {
		if !m[v] {
			m[v] = true
			datelist = append(datelist, v)
		}
	}

	return datelist, nil
}
