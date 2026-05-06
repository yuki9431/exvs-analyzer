package scraper

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	maxParallelism = 3

	// requestDelay はリクエスト完了後の待機時間（サーバー負荷軽減用）
	requestDelay = 300 * time.Millisecond
)

// ErrAccessDenied はサーバーからアクセス拒否(403)された場合のエラー
var ErrAccessDenied = errors.New("サーバーからアクセスが拒否されました")

// ErrUnauthorized はサーバーから認証拒否(401)された場合のエラー
var ErrUnauthorized = errors.New("認証が無効です")

// ErrServerError はサーバー内部エラー(5xx)の場合のエラー
var ErrServerError = errors.New("サーバーでエラーが発生しています")

// ErrNotFound はページが見つからない(404)場合のエラー
var ErrNotFound = errors.New("ページが見つかりません")

// ErrHTTPRequestFailed はHTTPリクエストが失敗した場合のエラー
var ErrHTTPRequestFailed = errors.New("データ取得中にHTTPエラーが発生しました")

// TagPartner はタッグ戦歴ページから取得した固定相方情報
type TagPartner struct {
	TeamName   string
	PlayerName string
}

// dailyLink はrankpageから収集した日別ページ情報
type dailyLink struct {
	date     string
	url      string
	shopName string // プレイ店舗名
}

// matchEntry は日別ページから収集した試合情報
type matchEntry struct {
	date      string
	hour      string
	wins      []string
	detailURL string
	shopName  string // プレイ店舗名（dailyLinkから引き継ぎ）
}

// stripQueryParam はURLからクエリパラメータを除去する
func stripQueryParam(rawURL string) string {
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
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

	// 403検出時に全処理を即座に打ち切るためのcontext
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Phase 2+3: 日別ページ収集→詳細ページ取得をパイプラインで並行実行
	// Phase 2で試合エントリが見つかり次第、Phase 3の詳細取得を開始する
	entryCh := make(chan matchEntry, 50)
	var streamErr error

	go func() {
		defer close(entryCh)
		streamErr = streamMatchEntries(ctx, cancel, m.HTTPClient.Jar, dailyLinks, since, entryCh)
	}()

	scores, detailErr := fetchDetailPagesStreaming(ctx, cancel, m.HTTPClient.Jar, entryCh, notify)

	// Phase 2のエラーを優先的に返す（403はより深刻なため）
	if streamErr != nil {
		return nil, nil, streamErr
	}
	// 403の場合は途中データとエラーを両方返す（呼び出し元で途中保存できるようにする）
	if detailErr != nil {
		if errors.Is(detailErr, ErrAccessDenied) && len(scores) > 0 {
			log.Printf("[INFO] Returning %d partial scores despite 403 error", len(scores))
			return scores, m.HTTPClient.Jar, detailErr
		}
		return nil, nil, detailErr
	}

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
		shopName := strings.TrimSpace(e.ChildText("span.ds-ib.tl-l.col-stand.fz-ss"))
		links = append(links, dailyLink{date: date, url: link, shopName: shopName})
	})

	c.Visit(mobileRankpage)

	if accessDenied {
		return nil, ErrAccessDenied
	}
	return links, nil
}

// streamMatchEntries は複数の日別ページから試合エントリを並列で収集し、チャネルにストリーミングする
// HTTPエラーが1件でもあればエラーを返す。403の場合はErrAccessDeniedを返し即座にキャンセルする
func streamMatchEntries(ctx context.Context, cancel context.CancelFunc, jar http.CookieJar, links []dailyLink, since time.Time, out chan<- matchEntry) error {
	if len(links) == 0 {
		return nil
	}

	sem := make(chan struct{}, maxParallelism)
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		totalPages int
		errorCount int
		has403     bool
	)

	for _, dl := range links {
		// キャンセル済みなら新規goroutineを起動しない
		select {
		case <-ctx.Done():
			break
		default:
		}
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(dl dailyLink) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			entries, err := collectMatchEntries(jar, dl, since)
			mu.Lock()
			totalPages++
			if err != nil {
				errorCount++
				if errors.Is(err, ErrAccessDenied) {
					has403 = true
				}
				cancel()
			}
			mu.Unlock()

			for _, e := range entries {
				select {
				case <-ctx.Done():
					return
				case out <- e:
				}
			}
			time.Sleep(requestDelay)
		}(dl)
	}

	wg.Wait()

	if has403 {
		return ErrAccessDenied
	}
	if errorCount > 0 {
		return fmt.Errorf("日別ページ取得で%w: %d/%d件がエラー", ErrHTTPRequestFailed, errorCount, totalPages)
	}
	return nil
}

// collectMatchEntries は単一の日別ページから試合エントリを収集する（ページネーション対応）
// since以前の試合が出たらページネーションを早期終了する
// HTTPエラーが発生した場合はエラーを返す（403の場合はErrAccessDenied）
func collectMatchEntries(jar http.CookieJar, dl dailyLink, since time.Time) ([]matchEntry, error) {
	var entries []matchEntry
	var httpErr error
	stopPagination := false

	c := colly.NewCollector(colly.AllowedDomains(vsmobile))
	c.SetCookieJar(jar)

	c.OnError(func(r *colly.Response, err error) {
		// 403は最も重要なエラーなので、一度記録したら上書きしない
		if errors.Is(httpErr, ErrAccessDenied) {
			return
		}
		if r.StatusCode == http.StatusForbidden {
			httpErr = ErrAccessDenied
			log.Printf("[ERROR] collectMatchEntries: 403 Forbidden url=%s err=%v", r.Request.URL, err)
		} else {
			httpErr = fmt.Errorf("リクエストエラー: url=%s: %w", r.Request.URL, err)
			log.Printf("[ERROR] collectMatchEntries: HTTP %d url=%s err=%v", r.StatusCode, r.Request.URL, err)
		}
	})

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
			shopName:  dl.shopName,
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
	return entries, httpErr
}

// fetchDetailPagesStreaming はチャネルから試合エントリを受信しつつ詳細ページを並列取得する
// HTTPエラーが1件でもあればエラーを返す。403の場合はErrAccessDeniedを返し即座にキャンセルする
func fetchDetailPagesStreaming(ctx context.Context, cancel context.CancelFunc, jar http.CookieJar, entryCh <-chan matchEntry, notify func(int, int)) (model.DatedScores, error) {
	// まず全エントリを収集してtotalを確定させる（キャンセル時はチャネルが閉じるまで待つ）
	var entries []matchEntry
	for entry := range entryCh {
		select {
		case <-ctx.Done():
			// チャネルに残ったエントリを捨てて終了を待つ
			for range entryCh {
			}
			// drain完了後、収集済みエントリで詳細取得に進む
			break
		default:
			entries = append(entries, entry)
		}
	}

	// streamMatchEntries側で403が発生していた場合
	if ctx.Err() != nil {
		return nil, ErrAccessDenied
	}

	total := len(entries)
	if total == 0 {
		return nil, nil
	}

	var (
		scores     model.DatedScores
		mu         sync.Mutex
		wg         sync.WaitGroup
		processed  int
		errorCount int
		has403     bool
	)

	sem := make(chan struct{}, maxParallelism)

	for _, entry := range entries {
		// キャンセル済みなら新規リクエストを発行しない
		select {
		case <-ctx.Done():
			break
		default:
		}
		if ctx.Err() != nil {
			break
		}

		select {
		case <-ctx.Done():
			break
		case sem <- struct{}{}:
		}
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(e matchEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			// キャンセル済みならスキップ
			if ctx.Err() != nil {
				return
			}

			parsed, err := fetchSingleDetail(ctx, jar, e)
			mu.Lock()
			scores = append(scores, parsed...)
			processed++
			if err != nil {
				errorCount++
				if errors.Is(err, ErrAccessDenied) {
					has403 = true
				}
				cancel()
			}
			current := processed
			mu.Unlock()

			notify(current, total)
			time.Sleep(requestDelay)
		}(entry)
	}

	wg.Wait()

	if has403 {
		log.Printf("[WARN] 403 detected during detail fetch: %d/%d pages completed, returning partial data", processed, total)
		return scores, ErrAccessDenied
	}
	if errorCount > 0 {
		return nil, fmt.Errorf("詳細ページ取得で%w: %d/%d件がエラー", ErrHTTPRequestFailed, errorCount, total)
	}
	return scores, nil
}

// fetchSingleDetail は単一の試合詳細ページをnet/http+goqueryで取得しスコアを返す
// HTTPエラーが発生した場合はエラーを返す（403の場合はErrAccessDenied）
func fetchSingleDetail(ctx context.Context, jar http.CookieJar, e matchEntry) (model.DatedScores, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.detailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエスト作成失敗: url=%s: %w", e.detailURL, err)
	}

	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("[ERROR] fetchSingleDetail: リクエスト失敗 url=%s err=%v", e.detailURL, err)
		return nil, fmt.Errorf("リクエスト失敗: url=%s: %w", e.detailURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[ERROR] fetchSingleDetail: HTTP %d url=%s", resp.StatusCode, e.detailURL)
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, ErrUnauthorized
		case resp.StatusCode == http.StatusForbidden:
			return nil, ErrAccessDenied
		case resp.StatusCode == http.StatusNotFound:
			return nil, ErrNotFound
		case resp.StatusCode >= 500:
			return nil, fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
		default:
			return nil, fmt.Errorf("HTTPエラー %d: url=%s", resp.StatusCode, e.detailURL)
		}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[ERROR] fetchSingleDetail: HTML解析失敗 url=%s err=%v", e.detailURL, err)
		return nil, fmt.Errorf("HTML解析失敗: url=%s: %w", e.detailURL, err)
	}

	var scores model.DatedScores
	doc.Find("div.panel_area").Each(func(_ int, s *goquery.Selection) {
		scores = parseDetailPage(s, e.date, e.hour, e.wins, e.shopName)
	})
	return scores, nil
}

// parseDetailPage は試合詳細ページからスコアを抽出する
func parseDetailPage(s *goquery.Selection, date, hour string, wins []string, shopName string) model.DatedScores {
	var scores model.DatedScores

	// スコアタブ(panel3)からの既存データ
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

	// メンバータブ(panel1)からの追加データ
	masteries := parseMasteries(s)
	teamNames := parseTeamNames(s)
	titleImages := attrsFromSelection(s, "#panel1 img.title-plv-img", "src")
	titleBadges := attrsFromSelection(s, "#panel1 img.title-plv-badge", "src")
	profileLinks := attrsFromSelection(s, "#panel1 li.item > a.right-arrow", "href")
	gradeImages := attrsFromSelection(s, "#panel1 img.class-img", "data-original")
	rankingImages := attrsFromSelection(s, "#panel3 img.ranking-img", "src")

	// 試合経過タブ(panel2)からのタイムラインデータ
	timeline := parseMatchTimeline(s)

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

		mastery := ""
		if i < len(masteries) {
			mastery = masteries[i]
		}

		// チーム名: player 1,2 → team1, player 3,4 → team2
		teamName := ""
		teamIdx := i / 2
		if teamIdx < len(teamNames) {
			teamName = teamNames[teamIdx]
		}

		titleImage := ""
		if i < len(titleImages) {
			titleImage = titleImages[i]
		}
		titleBadge := ""
		if i < len(titleBadges) {
			titleBadge = titleBadges[i]
		}
		profileLink := ""
		if i < len(profileLinks) {
			profileLink = profileLinks[i]
		}
		// クラス画像は各プレイヤーに2枚ずつ (シャッフル階級・固定階級)
		shuffleGrade := ""
		teamGrade := ""
		gradeIdx := i * 2
		if gradeIdx < len(gradeImages) {
			shuffleGrade = stripQueryParam(gradeImages[gradeIdx])
		}
		if gradeIdx+1 < len(gradeImages) {
			teamGrade = stripQueryParam(gradeImages[gradeIdx+1])
		}
		// スコア順位バッジ (4位はテキスト表示のため画像がない場合あり)
		rankingImage := ""
		if i < len(rankingImages) {
			rankingImage = rankingImages[i]
		}

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
				Mastery:        mastery,
				TeamName:       teamName,
				TitleImage:     titleImage,
				TitleBadge:     titleBadge,
				ProfileLink:    profileLink,
				ShuffleGrade:   shuffleGrade,
				TeamGrade:      teamGrade,
				RankingImage:   rankingImage,
				ShopName:       shopName,
			},
		}

		// タイムラインはPlayerNo==1のときのみセット（4人で共有データ）
		if i == 0 && timeline != nil {
			result.MatchTimeline = timeline
		}

		scores = append(scores, result)
	}

	return scores
}

// parseMasteries はメンバータブからランク情報を抽出する
// span.masteryのclass属性から"mastery"以外のクラス名を取得する
func parseMasteries(s *goquery.Selection) []string {
	var masteries []string
	s.Find("#panel1 span.mastery").Each(func(_ int, el *goquery.Selection) {
		classes, exists := el.Attr("class")
		if !exists {
			masteries = append(masteries, "")
			return
		}
		rank := ""
		for _, c := range strings.Fields(classes) {
			if c != "mastery" {
				rank = c
				break
			}
		}
		masteries = append(masteries, rank)
	})
	return masteries
}

// parseTeamNames はメンバータブからチーム名を抽出する
// panel1のbox内h3にあるtag-nameを取得（2チーム分）
func parseTeamNames(s *goquery.Selection) []string {
	var names []string
	s.Find("#panel1 > div.box > h3 p.tag-name").Each(func(_ int, el *goquery.Selection) {
		names = append(names, strings.TrimSpace(el.Text()))
	})
	return names
}

// vis.js DataSetパーサー用の正規表現
var (
	rePush      = regexp.MustCompile(`dataset\.push\(\{(.+?)\}\)`)
	reGroup     = regexp.MustCompile(`group:\s*"([^"]+)"`)
	reClassName = regexp.MustCompile(`className:\s*'([^']+)'`)
	reType      = regexp.MustCompile(`type:\s*'([^']+)'`)
	reStartTime = regexp.MustCompile(`var\s+start_time\s*=\s*new\s+Date\(0,\s*0,\s*0,\s*(\d+),\s*(\d+),\s*(\d+)\)`)
	reEndTime   = regexp.MustCompile(`var\s+end_time\s*=\s*new\s+Date\(0,\s*0,\s*0,\s*(\d+),\s*(\d+),\s*(\d+)\)`)
	reGameOver  = regexp.MustCompile(`addCustomTime\(new\s+Date\(0,\s*0,\s*0,\s*(\d+),\s*(\d+),\s*(\d+)\),\s*'game-over'\)`)
)

// datePartsToSec はvis.jsのDate(0,0,0,min,sec,centisec)を秒に変換する
func datePartsToSec(minStr, secStr, centiStr string) float64 {
	min, _ := strconv.Atoi(minStr)
	sec, _ := strconv.Atoi(secStr)
	centi, _ := strconv.Atoi(centiStr)
	return float64(min)*60 + float64(sec) + float64(centi)/100.0
}

// parseMatchTimeline は試合経過タブからvis.jsのタイムラインデータを解析する
func parseMatchTimeline(s *goquery.Selection) *model.MatchTimeline {
	// panel2内のscriptタグからJavaScriptコードを取得
	var scriptText string
	s.Find("#panel2 script").Each(func(_ int, el *goquery.Selection) {
		text := el.Text()
		if strings.Contains(text, "dataset.push") {
			scriptText = text
		}
	})

	if scriptText == "" {
		return nil
	}

	var events []model.MatchEvent

	// scriptTextを行ごとに処理し、start_time/end_time変数とdataset.pushを対応付ける
	lines := strings.Split(scriptText, "\n")
	var currentStart float64
	var currentEnd float64
	var hasEnd bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if m := reStartTime.FindStringSubmatch(line); m != nil {
			currentStart = datePartsToSec(m[1], m[2], m[3])
			hasEnd = false
			currentEnd = 0
			continue
		}

		if m := reEndTime.FindStringSubmatch(line); m != nil {
			currentEnd = datePartsToSec(m[1], m[2], m[3])
			hasEnd = true
			continue
		}

		if m := rePush.FindStringSubmatch(line); m != nil {
			content := m[1]

			groupMatch := reGroup.FindStringSubmatch(content)
			if groupMatch == nil {
				continue
			}

			event := model.MatchEvent{
				Group:    groupMatch[1],
				StartSec: currentStart,
			}

			if hasEnd {
				event.EndSec = currentEnd
			}

			if classMatch := reClassName.FindStringSubmatch(content); classMatch != nil {
				event.ClassName = classMatch[1]
			}

			if typeMatch := reType.FindStringSubmatch(content); typeMatch != nil && typeMatch[1] == "point" {
				event.IsPoint = true
			}

			events = append(events, event)
		}
	}

	if len(events) == 0 {
		return nil
	}

	timeline := &model.MatchTimeline{
		Events: events,
	}

	// ゲーム終了時間を取得
	if m := reGameOver.FindStringSubmatch(scriptText); m != nil {
		timeline.GameEndSec = datePartsToSec(m[1], m[2], m[3])
	}

	return timeline
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

	c.OnHTML("li.item", func(e *colly.HTMLElement) {
		teamName := strings.TrimSpace(e.ChildText("p.tag-name"))
		playerName := strings.TrimSpace(e.ChildText("p.ml-ss"))

		if playerName != "" {
			partners = append(partners, TagPartner{
				TeamName:   teamName,
				PlayerName: playerName,
			})
		}
	})

	c.Visit(mobileTagPage)
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
