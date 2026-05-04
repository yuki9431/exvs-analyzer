package scraper

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/yuki9431/exvs-analyzer/internal/model"
)

const (
	vsmobile         = "web.vsmobile.jp"
	mobileRankpage   = "https://web.vsmobile.jp/exvs2ib/results/classmatch/fight"
	mobileTagPage    = "https://web.vsmobile.jp/exvs2ib/results/classmatch/tag"
	mobileMSUsedRate = "https://web.vsmobile.jp/exvs2ib/ranking/ms_used_rate"

	// maxParallelism はバンナムサーバーへの最大同時リクエスト数
	maxParallelism = 15
)

// ErrAccessDenied はサーバーからアクセス拒否(403)された場合のエラー
var ErrAccessDenied = errors.New("サーバーからアクセスが拒否されました")

// TagPartner はタッグ戦歴ページから取得した固定相方情報
type TagPartner struct {
	TeamName   string
	PlayerName string
}

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

// Scraping はスクレイピング処理を実行し、DatedScoresとログイン済みCookieJarを返す
// 日別ページ収集と詳細ページ取得をパイプラインで並行実行し、高速化を図る
func Scraping(username, password string, since time.Time, onProgress ...ProgressFunc) (model.DatedScores, http.CookieJar, error) {
	notify := func(current, total int) {
		if len(onProgress) > 0 && onProgress[0] != nil {
			onProgress[0](current, total)
		}
	}

	m := NewClient(username, password)
	if err := m.Login(); err != nil {
		return nil, nil, fmt.Errorf("ログインに失敗: %w", err)
	}

	// Phase 1: rankpageから日別ページURLを収集
	dailyLinks, err := collectDailyLinks(m.HTTPClient.Jar, since)
	if err != nil {
		return nil, nil, err
	}

	// Phase 2+3: 日別ページ収集→詳細ページ取得をパイプラインで並行実行
	// Phase 2で試合エントリが見つかり次第、Phase 3の詳細取得を開始する
	entryCh := make(chan matchEntry, 50)

	go func() {
		defer close(entryCh)
		streamMatchEntries(m.HTTPClient.Jar, dailyLinks, since, entryCh)
	}()

	scores := fetchDetailPagesStreaming(m.HTTPClient.Jar, entryCh, notify)

	// 日時降順・プレイヤーNo昇順でソート
	sort.Slice(scores, func(i, j int) bool {
		if !scores[i].Datetime.Equal(scores[j].Datetime) {
			return scores[i].Datetime.After(scores[j].Datetime)
		}
		return scores[i].PlayerNo < scores[j].PlayerNo
	})

	return scores, m.HTTPClient.Jar, nil
}

// collectDailyLinks はrankpageから日別ページのURLを収集する
func collectDailyLinks(jar http.CookieJar, since time.Time) ([]dailyLink, error) {
	var links []dailyLink

	c := colly.NewCollector(colly.AllowedDomains(vsmobile))
	c.SetCookieJar(jar)

	var accessDenied bool
	c.OnResponse(func(r *colly.Response) {
		if r.StatusCode == http.StatusForbidden {
			accessDenied = true
		}
	})

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

	if accessDenied {
		return nil, ErrAccessDenied
	}
	return links, nil
}

// streamMatchEntries は複数の日別ページから試合エントリを並列で収集し、チャネルにストリーミングする
func streamMatchEntries(jar http.CookieJar, links []dailyLink, since time.Time, out chan<- matchEntry) {
	sem := make(chan struct{}, maxParallelism)
	var wg sync.WaitGroup

	for _, dl := range links {
		wg.Add(1)
		go func(dl dailyLink) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			entries := collectMatchEntries(jar, dl, since)
			for _, e := range entries {
				out <- e
			}
		}(dl)
	}

	wg.Wait()
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

// fetchDetailPagesStreaming はチャネルから試合エントリを受信しつつ詳細ページを並列取得する
func fetchDetailPagesStreaming(jar http.CookieJar, entryCh <-chan matchEntry, notify func(int, int)) model.DatedScores {
	// まず全エントリを収集してtotalを確定させる
	var entries []matchEntry
	for entry := range entryCh {
		entries = append(entries, entry)
	}

	total := len(entries)
	if total == 0 {
		return nil
	}

	var (
		scores    model.DatedScores
		mu        sync.Mutex
		wg        sync.WaitGroup
		processed int
	)

	sem := make(chan struct{}, maxParallelism)

	for _, entry := range entries {
		sem <- struct{}{}
		wg.Add(1)
		go func(e matchEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			parsed := fetchSingleDetail(jar, e)
			mu.Lock()
			scores = append(scores, parsed...)
			processed++
			current := processed
			mu.Unlock()

			notify(current, total)
		}(entry)
	}

	wg.Wait()
	return scores
}

// fetchSingleDetail は単一の試合詳細ページをnet/http+goqueryで取得しスコアを返す
func fetchSingleDetail(jar http.CookieJar, e matchEntry) model.DatedScores {
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	resp, err := client.Get(e.detailURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	var scores model.DatedScores
	doc.Find("div.panel_area").Each(func(_ int, s *goquery.Selection) {
		scores = parseDetailPage(s, e.date, e.hour, e.wins)
	})
	return scores
}

// parseDetailPage は試合詳細ページからスコアを抽出する
func parseDetailPage(s *goquery.Selection, date, hour string, wins []string) model.DatedScores {
	var scores model.DatedScores

	selectorLeftValue := "div.w45.pr-ss > dl > dd"
	selectorRightValue := "div.w55 > dl > dd"
	selectorCity := "div.w80.ta-r > p.col-stand"
	selectorName := "p.mb-ss.fz-m > span.name"
	selectorMSImage := "#panel3 img.item-icon-img"

	cities := textsFromSelection(s, selectorCity)
	names := textsFromSelection(s, selectorName)
	msImages := attrsFromSelection(s, selectorMSImage, "data-original")
	leftValue := textsFromSelection(s, selectorLeftValue)
	rightValue := textsFromSelection(s, selectorRightValue)

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

// textsFromSelection はgoquery.Selectionから指定セレクタの子要素テキストを収集する
func textsFromSelection(s *goquery.Selection, selector string) []string {
	var texts []string
	s.Find(selector).Each(func(_ int, el *goquery.Selection) {
		texts = append(texts, strings.TrimSpace(el.Text()))
	})
	return texts
}

// attrsFromSelection はgoquery.Selectionから指定セレクタの属性値を収集する
func attrsFromSelection(s *goquery.Selection, selector, attr string) []string {
	var attrs []string
	s.Find(selector).Each(func(_ int, el *goquery.Selection) {
		if val, exists := el.Attr(attr); exists {
			attrs = append(attrs, val)
		}
	})
	return attrs
}

// ScrapeTagPartners はタッグ戦歴ページからチーム名と相方のプレイヤー名を取得する
func ScrapeTagPartners(jar http.CookieJar) []TagPartner {
	var partners []TagPartner

	c := colly.NewCollector(colly.AllowedDomains(vsmobile))
	c.SetCookieJar(jar)

	c.OnResponse(func(r *colly.Response) {
		fmt.Printf("[DEBUG] ScrapeTagPartners status=%d url=%s bodyLen=%d\n", r.StatusCode, r.Request.URL, len(r.Body))
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("[DEBUG] ScrapeTagPartners error: %v, status=%d\n", err, r.StatusCode)
	})

	c.OnHTML("li.item", func(e *colly.HTMLElement) {
		teamName := strings.TrimSpace(e.ChildText("p.tag-name"))
		playerName := strings.TrimSpace(e.ChildText("p.ml-ss"))
		fmt.Printf("[DEBUG] ScrapeTagPartners item: team=%q player=%q\n", teamName, playerName)

		if playerName != "" {
			partners = append(partners, TagPartner{
				TeamName:   teamName,
				PlayerName: playerName,
			})
		}
	})

	c.Visit(mobileTagPage)
	fmt.Printf("[DEBUG] ScrapeTagPartners result: %d partners found\n", len(partners))
	return partners
}

// ScrapeMSList は機体使用率ランキングページから画像URLと機体名の一覧を取得する
func ScrapeMSList(username, password string) ([]model.MSInfo, error) {
	var msList []model.MSInfo
	seen := make(map[string]bool)

	m := NewClient(username, password)
	if err := m.Login(); err != nil {
		return nil, fmt.Errorf("ログインに失敗: %w", err)
	}

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

	return msList, nil
}
