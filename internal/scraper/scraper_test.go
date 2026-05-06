// scraper_test.go — internal/scraper パッケージのユニットテスト
//
// テスト対象と観点:
//   - parseNumber: カンマ区切りの数値文字列→int変換。空文字や非数値も確認
//   - parseMasteries: span.masteryのclass属性からランク情報を抽出
//   - parseTeamNames: panel1のbox内h3からチーム名を抽出
//   - parseMatchTimeline: vis.js DataSetスクリプトからタイムラインイベントを解析
//   - datePartsToSec: Date(0,0,0,min,sec,centi)→秒変換
//
// 外部サイトに接続する Scraping/ScrapeMSList はテスト対象外。
//
// 実行方法:
//
//	make test
package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,234", 1234},
		{"100", 100},
		{"0", 0},
		{"12,345,678", 12345678},
		{"", 0},
		{"abc", 0},
		{"score: 1,500pt", 1500},
	}

	for _, tt := range tests {
		got := parseNumber(tt.input)
		if got != tt.want {
			t.Errorf("parseNumber(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDatePartsToSec(t *testing.T) {
	tests := []struct {
		min, sec, centi string
		want            float64
	}{
		{"0", "41", "75", 41.75},
		{"1", "15", "97", 75.97},
		{"2", "23", "0", 143.0},
		{"0", "0", "0", 0.0},
	}

	for _, tt := range tests {
		got := datePartsToSec(tt.min, tt.sec, tt.centi)
		if got != tt.want {
			t.Errorf("datePartsToSec(%s, %s, %s) = %f, want %f", tt.min, tt.sec, tt.centi, got, tt.want)
		}
	}
}

func TestParseMasteries(t *testing.T) {
	html := `<div id="panel1">
		<div class="box">
			<ul>
				<li class="item"><div class="w20"><span class="mastery master"></span></div></li>
				<li class="item"><div class="w20"><span class="mastery silver5"></span></div></li>
			</ul>
		</div>
		<div class="box">
			<ul>
				<li class="item"><div class="w20"><span class="mastery gold2"></span></div></li>
				<li class="item"><div class="w20"><span class="mastery silver1"></span></div></li>
			</ul>
		</div>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	masteries := parseMasteries(doc.Selection)

	if len(masteries) != 4 {
		t.Fatalf("got %d masteries, want 4", len(masteries))
	}

	expected := []string{"master", "silver5", "gold2", "silver1"}
	for i, want := range expected {
		if masteries[i] != want {
			t.Errorf("mastery[%d]: got %q, want %q", i, masteries[i], want)
		}
	}
}

func TestParseTeamNames(t *testing.T) {
	html := `<div id="panel1">
		<div class="box">
			<h3><p class="tag-name fw-n ds-ib">チームA</p></h3>
			<ul><li class="item"></li><li class="item"></li></ul>
		</div>
		<div class="box">
			<h3><p class="tag-name fw-n ds-ib">チームB</p></h3>
			<ul><li class="item"></li><li class="item"></li></ul>
		</div>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	names := parseTeamNames(doc.Selection)

	if len(names) != 2 {
		t.Fatalf("got %d team names, want 2", len(names))
	}
	if names[0] != "チームA" {
		t.Errorf("team[0]: got %q, want %q", names[0], "チームA")
	}
	if names[1] != "チームB" {
		t.Errorf("team[1]: got %q, want %q", names[1], "チームB")
	}
}

func TestParseMatchTimeline(t *testing.T) {
	html := `<div id="panel2">
<script>
    var dataset = [];
                var start_time = new Date(0, 0, 0, 0, 41, 75);
                    var end_time = new Date(0, 0, 0, 0, 46, 25);
            dataset.push({ id: 0,  group: "team1-1", start: start_time, end: end_time, className:'ex' });
                        var start_time = new Date(0, 0, 0, 0, 46, 25);
                    var end_time = new Date(0, 0, 0, 0, 54, 82);
            dataset.push({ id: 1,  group: "team1-1", start: start_time, end: end_time, className:'exbst-f' });
                        var start_time = new Date(0, 0, 0, 0, 57, 48);
                    dataset.push({ id: 2,  group: "team1-1", start: start_time, type:'point' });
                        var start_time = new Date(0, 0, 0, 1, 40, 88);
                    dataset.push({ id: 22,  group: "team2-2", start: start_time, type:'point' });

    timeline.addCustomTime(new Date(0, 0, 0, 0, 0, 0),'game-start');
    timeline.addCustomTime(new Date(0, 0, 0, 2, 23, 0),'game-over');
</script>
</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	tl := parseMatchTimeline(doc.Selection)

	if tl == nil {
		t.Fatal("expected non-nil timeline")
	}

	if len(tl.Events) != 4 {
		t.Fatalf("got %d events, want 4", len(tl.Events))
	}

	// EX available event
	ev0 := tl.Events[0]
	if ev0.Group != "team1-1" {
		t.Errorf("event[0].Group: got %q, want %q", ev0.Group, "team1-1")
	}
	if ev0.ClassName != "ex" {
		t.Errorf("event[0].ClassName: got %q, want %q", ev0.ClassName, "ex")
	}
	if ev0.StartSec != 41.75 {
		t.Errorf("event[0].StartSec: got %f, want 41.75", ev0.StartSec)
	}
	if ev0.EndSec != 46.25 {
		t.Errorf("event[0].EndSec: got %f, want 46.25", ev0.EndSec)
	}
	if ev0.IsPoint {
		t.Error("event[0].IsPoint should be false")
	}

	// Burst activation event
	ev1 := tl.Events[1]
	if ev1.ClassName != "exbst-f" {
		t.Errorf("event[1].ClassName: got %q, want %q", ev1.ClassName, "exbst-f")
	}

	// Death event (point)
	ev2 := tl.Events[2]
	if !ev2.IsPoint {
		t.Error("event[2].IsPoint should be true")
	}
	if ev2.StartSec != 57.48 {
		t.Errorf("event[2].StartSec: got %f, want 57.48", ev2.StartSec)
	}
	if ev2.EndSec != 0 {
		t.Errorf("event[2].EndSec: got %f, want 0", ev2.EndSec)
	}

	// Game end time
	if tl.GameEndSec != 143.0 {
		t.Errorf("GameEndSec: got %f, want 143.0", tl.GameEndSec)
	}
}

func TestParseMatchTimeline_Empty(t *testing.T) {
	html := `<div id="panel2"><script>var x = 1;</script></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	tl := parseMatchTimeline(doc.Selection)
	if tl != nil {
		t.Error("expected nil timeline for script without dataset.push")
	}
}
