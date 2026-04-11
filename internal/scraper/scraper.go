package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/yuki9431/exvs-analyzer/internal/model"
)

const (
	vsmobile         = "web.vsmobile.jp"
	mobileRankpage   = "https://web.vsmobile.jp/exvs2ib/results/classmatch/fight"
	mobileMSUsedRate = "https://web.vsmobile.jp/exvs2ib/ranking/ms_used_rate"

	// maxParallelism はバンナムサーバーへの最大同時リクエスト数
	maxParallelism = 3
)

// dailyLink はrankpageから収集した日別ページ情報
type dailyLink struct {
	date string
	url  string
}

// matchEntry は日別ページから収集した試合情報
type matchEntry struct {
	date      string
	hour      string
	wins      []string
	detailURL string
}

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

// ProgressFunc はスクレイピングの進捗を通知するコールバック型
type ProgressFunc func(current, total int)

// Scraping はスクレイピング処理を実行し、DatedScoresを返す
// 日別ページと詳細ページを並列で取得し、高速化を図る
func Scraping(username, password string, since time.Time, onProgress ...ProgressFunc) model.DatedScores {
	notify := func(current, total int) {
		if len(onProgress) > 0 && onProgress[0] != nil {
			onProgress[0](current, total)
		}
	}

	m := NewClient(username, password)
	m.Login()

	// Phase 1: rankpageから日別ページURLを収集
	dailyLinks := collectDailyLinks(m.HTTPClient.Jar, since)

	// Phase 2: 日別ページから試合エントリを並列収集
	entries := collectAllMatchEntries(m.HTTPClient.Jar, dailyLinks, since)

	// 日時降順でソートして元の取得順序を再現
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := time.Parse("2006/01/02 15:04", entries[i].date+" "+entries[i].hour)
		tj, _ := time.Parse("2006/01/02 15:04", entries[j].date+" "+entries[j].hour)
		return ti.After(tj)
	})

	// Phase 3: 試合詳細ページを並列取得
	scores := fetchDetailPages(m.HTTPClient.Jar, entries, notify)

	// 日時降順・プレイヤーNo昇順でソートして元の順序を保つ
	sort.Slice(scores, func(i, j int) bool {
		if !scores[i].Datetime.Equal(scores[j].Datetime) {
			return scores[i].Datetime.After(scores[j].Datetime)
		}
		return scores[i].PlayerNo < scores[j].PlayerNo
	})

	return scores
}

// collectDailyLinks はrankpageから日別ページのURLを収集する
func collectDailyLinks(jar http.CookieJar, since time.Time) []dailyLink {
	var links []dailyLink

	c := colly.NewCollector(colly.AllowedDomains(vsmobile))
	c.SetCookieJar(jar)

	c.OnHTML("li.item", func(e *colly.HTMLElement) {
		r := regexp.MustCompile(`\(.*`)
		date := r.ReplaceAllString(e.ChildText("p.datetime.fz-ss"), "")

		if !since.IsZero() {
			d, err := time.Parse("2006/01/02", date)
			if err == nil && d.Before(since.Truncate(24*time.Hour)) {
				return
			}
		}

		link := e.Request.AbsoluteURL(e.ChildAttr("a", "href"))
		links = append(links, dailyLink{date: date, url: link})
	})

	c.Visit(mobileRankpage)
	return links
}

// collectAllMatchEntries は複数の日別ページから試合エントリを並列で収集する
func collectAllMatchEntries(jar http.CookieJar, links []dailyLink, since time.Time) []matchEntry {
	var (
		allEntries []matchEntry
		mu         sync.Mutex
	)

	sem := make(chan struct{}, maxParallelism)
	var wg sync.WaitGroup

	for _, dl := range links {
		wg.Add(1)
		go func(dl dailyLink) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			entries := collectMatchEntries(jar, dl, since)
			mu.Lock()
			allEntries = append(allEntries, entries...)
			mu.Unlock()
		}(dl)
	}

	wg.Wait()
	return allEntries
}

// collectMatchEntries は単一の日別ページから試合エントリを収集する（ページネーション対応）
// since以前の試合が出たらページネーションを早期終了する
func collectMatchEntries(jar http.CookieJar, dl dailyLink, since time.Time) []matchEntry {
	var entries []matchEntry
	stopPagination := false

	c := colly.NewCollector(colly.AllowedDomains(vsmobile))
	c.SetCookieJar(jar)

	c.OnHTML("li.item", func(e *colly.HTMLElement) {
		hour := e.ChildText("p.datetime.fz-ss")

		if !since.IsZero() {
			t, err := time.Parse("2006/01/02 15:04", dl.date+" "+hour)
			if err == nil && !t.After(since) {
				stopPagination = true
				return
			}
		}

		var wins []string
		if e.ChildAttr("a", "class") == "right-arrow vs-detail win" {
			wins = []string{"win", "win", "lose", "lose"}
		} else {
			wins = []string{"lose", "lose", "win", "win"}
		}

		link := e.Request.AbsoluteURL(e.ChildAttr("a", "href"))
		entries = append(entries, matchEntry{
			date:      dl.date,
			hour:      hour,
			wins:      wins,
			detailURL: link,
		})
	})

	c.OnHTML("div.block.control", func(e *colly.HTMLElement) {
		if stopPagination {
			return
		}
		// 「>」(次へ)ボタンは末尾から2番目のリンク
		links := e.ChildAttrs("ul.clearfix > li > a", "href")
		if len(links) >= 2 {
			nextLink := links[len(links)-2]
			if nextLink != "javascript:void(0);" {
				c.Visit(e.Request.AbsoluteURL(nextLink))
			}
		}
	})

	c.Visit(dl.url)
	return entries
}

// fetchDetailPages は試合詳細ページを並列で取得しDatedScoresを返す
func fetchDetailPages(jar http.CookieJar, entries []matchEntry, notify func(int, int)) model.DatedScores {
	var (
		scores model.DatedScores
		mu     sync.Mutex
	)

	c := colly.NewCollector(
		colly.AllowedDomains(vsmobile),
		colly.Async(true),
	)
	c.SetCookieJar(jar)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: maxParallelism,
	})

	total := len(entries)
	processed := 0

	c.OnHTML("div.panel_area", func(e *colly.HTMLElement) {
		date := e.Request.Ctx.Get("date")
		hour := e.Request.Ctx.Get("hour")
		wins := strings.Split(e.Request.Ctx.Get("wins"), ",")

		parsed := parseDetailPage(e, date, hour, wins)
		mu.Lock()
		scores = append(scores, parsed...)
		processed++
		current := processed
		mu.Unlock()

		notify(current, total)
	})

	for _, entry := range entries {
		ctx := colly.NewContext()
		ctx.Put("date", entry.date)
		ctx.Put("hour", entry.hour)
		ctx.Put("wins", strings.Join(entry.wins, ","))
		c.Request("GET", entry.detailURL, nil, ctx, nil)
	}

	c.Wait()
	return scores
}

// parseDetailPage は試合詳細ページからスコアを抽出する
func parseDetailPage(e *colly.HTMLElement, date, hour string, wins []string) model.DatedScores {
	var scores model.DatedScores

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

	layout := "2006/01/02 15:04"
	t := date + " " + hour
	datetime, _ := time.Parse(layout, t)

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
			Datetime: datetime,
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
