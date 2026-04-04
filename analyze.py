#!/usr/bin/env python3
"""
EXVS2IB 戦績分析スクリプト
CSVファイルを読み込み、定型分析レポートを出力する。
プレイヤー名はCSVのプレイヤーNo.1から自動取得する。
"""

import csv
import sys
from collections import defaultdict
from datetime import datetime


def get_season(dt):
    """日付からシーズン名を返す。偶数月開始の2ヶ月区切り。
    12-1月, 2-3月, 4-5月, 6-7月, 8-9月, 10-11月
    """
    m = dt.month
    # 12月は翌年の12-1月シーズン扱い
    if m == 12:
        start_month = 12
        year = dt.year
    elif m % 2 == 0:
        start_month = m
        year = dt.year
    else:
        start_month = m - 1
        year = dt.year if m != 1 else dt.year - 1
        if m == 1:
            start_month = 12

    end_month = start_month + 1 if start_month < 12 else 1
    end_year = year if start_month < 12 else year + 1

    return f"{year}年{start_month}月-{end_year}年{end_month}月" if start_month == 12 else f"{year}年{start_month}-{end_month}月"


def get_season_half(dt):
    """シーズン内で前半/後半を返す。開始月=前半、翌月=後半。"""
    m = dt.month
    if m == 12 or m % 2 == 0:
        return "前半"
    else:
        return "後半"


def load_csv(path):
    """CSVを読み込み、4行ごとに1試合としてグループ化する"""
    rows = []
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for r in reader:
            rows.append(r)

    matches = []
    for i in range(0, len(rows), 4):
        if i + 3 < len(rows):
            matches.append(rows[i : i + 4])
    return matches


def detect_player_name(matches):
    """プレイヤーNo.1の名前を自動検出する"""
    names = defaultdict(int)
    for m in matches:
        name = m[0]["プレイヤー名"].strip()
        if name:
            names[name] += 1
    return max(names, key=names.get) if names else ""


def get_player_ms(match):
    """プレイヤーの使用機体を返す。空欄の場合は'(不明)'"""
    ms = match[0]["機体名"].strip()
    return ms if ms else "(不明)"


def get_my_data(match):
    """自分(No.1)のデータを辞書で返す"""
    p = match[0]
    return {
        "win": p["勝利判定"] == "win",
        "ms": get_player_ms(match),
        "score": int(p["スコア"]),
        "kills": int(p["撃墜数"]),
        "deaths": int(p["被撃墜数"]),
        "dmg_given": int(p["与ダメージ"]),
        "dmg_taken": int(p["被ダメージ"]),
        "ex_dmg": int(p["EXダメージ"]),
        "datetime": datetime.strptime(p["試合日時"], "%Y-%m-%d %H:%M"),
        "partner_ms": match[1]["機体名"].strip() or "(不明)",
        "enemy_ms": [
            match[2]["機体名"].strip() or "(不明)",
            match[3]["機体名"].strip() or "(不明)",
        ],
    }


def avg(values):
    return sum(values) / len(values) if values else 0


def dmg_efficiency(data_list):
    total_given = sum(d["dmg_given"] for d in data_list)
    total_taken = sum(d["dmg_taken"] for d in data_list)
    return total_given / total_taken if total_taken > 0 else 0


def win_rate(data_list):
    if not data_list:
        return 0
    return sum(1 for d in data_list if d["win"]) / len(data_list) * 100


def print_header(title, level=1):
    if level == 1:
        print(f"\n{'=' * 60}")
        print(f"  {title}")
        print(f"{'=' * 60}")
    elif level == 2:
        print(f"\n--- {title} ---")
    elif level == 3:
        print(f"\n  【{title}】")


def analyze_basic_stats(data_list):
    """基本スタッツを出力"""
    n = len(data_list)
    wins = sum(1 for d in data_list if d["win"])
    wr = wins / n * 100 if n > 0 else 0

    print(f"  試合数:         {n}戦 ({wins}勝{n - wins}敗)")
    print(f"  勝率:           {wr:.1f}%")
    print(f"  平均与ダメージ: {avg([d['dmg_given'] for d in data_list]):.0f}")
    print(f"  平均被ダメージ: {avg([d['dmg_taken'] for d in data_list]):.0f}")
    print(f"  ダメージ効率:   {dmg_efficiency(data_list):.3f}")
    print(f"  平均撃墜:       {avg([d['kills'] for d in data_list]):.2f}")
    print(f"  平均被撃墜:     {avg([d['deaths'] for d in data_list]):.2f}")

    total_kills = sum(d["kills"] for d in data_list)
    total_deaths = sum(d["deaths"] for d in data_list)
    kd = total_kills / total_deaths if total_deaths > 0 else 0
    print(f"  K/D比:          {kd:.2f}")
    print(f"  平均EXダメージ: {avg([d['ex_dmg'] for d in data_list]):.0f}")


def analyze_win_loss_pattern(data_list):
    """勝ち/負け時のダメージ傾向"""
    wins = [d for d in data_list if d["win"]]
    losses = [d for d in data_list if not d["win"]]

    print(f"  {'':>20} {'勝ち':>10} {'負け':>10} {'差':>10}")
    print(f"  {'-' * 50}")

    metrics = [
        ("与ダメージ", "dmg_given"),
        ("被ダメージ", "dmg_taken"),
        ("撃墜", "kills"),
        ("被撃墜", "deaths"),
    ]
    for label, key in metrics:
        w_avg = avg([d[key] for d in wins]) if wins else 0
        l_avg = avg([d[key] for d in losses]) if losses else 0
        diff = w_avg - l_avg
        sign = "+" if diff >= 0 else ""
        print(f"  {label:>18} {w_avg:>10.1f} {l_avg:>10.1f} {sign}{diff:>9.1f}")

    w_eff = dmg_efficiency(wins) if wins else 0
    l_eff = dmg_efficiency(losses) if losses else 0
    print(f"  {'ダメ効率':>16} {w_eff:>10.3f} {l_eff:>10.3f} {w_eff - l_eff:>+10.3f}")


def analyze_enemy_matchup(data_list, min_matches=3):
    """敵機体との相性分析"""
    enemy_stats = defaultdict(list)
    for d in data_list:
        for ems in d["enemy_ms"]:
            enemy_stats[ems].append(d)

    results = []
    for ms, matches in enemy_stats.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            avg_given = avg([d["dmg_given"] for d in matches])
            avg_taken = avg([d["dmg_taken"] for d in matches])
            results.append((ms, len(matches), wr, eff, avg_given, avg_taken))

    strong = sorted([r for r in results if r[2] >= 60], key=lambda x: -x[2])
    weak = sorted([r for r in results if r[2] <= 40], key=lambda x: x[2])
    even = sorted([r for r in results if 40 < r[2] < 60], key=lambda x: -x[1])

    row_fmt = f"  {'機体名':>30} {'試合':>4} {'勝率':>6} {'ダメ効率':>8} {'与ダメ':>6} {'被ダメ':>6}"

    if strong:
        print_header("得意な相手 (勝率60%以上)", 3)
        print(row_fmt)
        for ms, n, wr, eff, gv, tk in strong:
            print(f"  {ms:>30} {n:>4} {wr:>5.0f}% {eff:>8.3f} {gv:>6.0f} {tk:>6.0f}")

    if weak:
        print_header("苦手な相手 (勝率40%以下)", 3)
        print(row_fmt)
        for ms, n, wr, eff, gv, tk in weak:
            print(f"  {ms:>30} {n:>4} {wr:>5.0f}% {eff:>8.3f} {gv:>6.0f} {tk:>6.0f}")

    if even:
        print_header("五分の相手 (勝率41-59%)", 3)
        print(row_fmt)
        for ms, n, wr, eff, gv, tk in even:
            print(f"  {ms:>30} {n:>4} {wr:>5.0f}% {eff:>8.3f} {gv:>6.0f} {tk:>6.0f}")


def analyze_partner(data_list, min_matches=3):
    """相方機体との相性分析"""
    partner_stats = defaultdict(list)
    for d in data_list:
        partner_stats[d["partner_ms"]].append(d)

    results = []
    for ms, matches in partner_stats.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            results.append((ms, len(matches), wr, eff))

    results.sort(key=lambda x: -x[2])
    print(f"  {'相方機体':>30} {'試合':>4} {'勝率':>6} {'ダメ効率':>8}")
    for ms, n, wr, eff in results:
        print(f"  {ms:>30} {n:>4} {wr:>5.0f}% {eff:>8.3f}")


def analyze_time_of_day(data_list):
    """時間帯別の勝率"""
    hourly = defaultdict(list)
    for d in data_list:
        hourly[d["datetime"].hour].append(d)

    print(f"  {'時間帯':>6} {'試合':>4} {'勝率':>6} {'ダメ効率':>8}")
    for hour in sorted(hourly.keys()):
        matches = hourly[hour]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        mark = " ★" if wr >= 70 else " ▼" if wr <= 40 else ""
        print(f"  {hour:>4}時台 {len(matches):>4} {wr:>5.1f}% {eff:>8.3f}{mark}")


def analyze_day_of_week(data_list):
    """曜日別の勝率（平日 vs 土日）"""
    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    daily = defaultdict(list)
    for d in data_list:
        dow = d["datetime"].weekday()
        daily[dow].append(d)

    weekday_data = [d for d in data_list if d["datetime"].weekday() < 5]
    weekend_data = [d for d in data_list if d["datetime"].weekday() >= 5]

    print(f"  平日: {len(weekday_data)}戦 勝率{win_rate(weekday_data):.1f}% ダメ効率{dmg_efficiency(weekday_data):.3f}")
    print(f"  土日: {len(weekend_data)}戦 勝率{win_rate(weekend_data):.1f}% ダメ効率{dmg_efficiency(weekend_data):.3f}")
    print()
    print(f"  {'曜日':>4} {'試合':>4} {'勝率':>6} {'ダメ効率':>8}")
    for dow in range(7):
        if dow in daily:
            matches = daily[dow]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            print(f"  {DOW_NAMES[dow]:>4} {len(matches):>4} {wr:>5.1f}% {eff:>8.3f}")


def analyze_daily_trend(data_list):
    """日別の勝率推移"""
    daily = defaultdict(list)
    for d in data_list:
        date_str = d["datetime"].strftime("%m/%d")
        daily[date_str].append(d)

    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    print(f"  {'日付':>6} {'曜日':>2} {'試合':>4} {'勝率':>6} {'ダメ効率':>8}")
    for date_str in sorted(daily.keys()):
        matches = daily[date_str]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        dow = matches[0]["datetime"].weekday()
        mark = " ★" if wr >= 70 else " ▼" if wr <= 45 else ""
        print(
            f"  {date_str:>6} ({DOW_NAMES[dow]}) {len(matches):>4} {wr:>5.1f}% {eff:>8.3f}{mark}"
        )


def analyze_season(data_list):
    """シーズン別分析（偶数月開始の2ヶ月区切り、自動判定）"""
    season_data = defaultdict(list)
    season_half = defaultdict(lambda: defaultdict(list))

    for d in data_list:
        s = get_season(d["datetime"])
        h = get_season_half(d["datetime"])
        season_data[s].append(d)
        season_half[s][h].append(d)

    for season_name in sorted(season_data.keys()):
        data = season_data[season_name]
        print_header(season_name, 3)
        print(f"  全体: {len(data)}戦 勝率{win_rate(data):.1f}% ダメ効率{dmg_efficiency(data):.3f}")

        for half_name in ["前半", "後半"]:
            hdata = season_half[season_name][half_name]
            if hdata:
                print(
                    f"  {half_name}: {len(hdata)}戦 勝率{win_rate(hdata):.1f}% ダメ効率{dmg_efficiency(hdata):.3f}"
                )


def analyze_deaths_impact(data_list):
    """被撃墜数と勝率の関係"""
    by_deaths = defaultdict(list)
    for d in data_list:
        deaths = d["deaths"]
        key = f"{deaths}回" if deaths <= 2 else "3回以上"
        by_deaths[key].append(d)

    print(f"  {'被撃墜':>8} {'試合':>4} {'勝率':>6} {'ダメ効率':>8}")
    for key in ["0回", "1回", "2回", "3回以上"]:
        if key in by_deaths:
            matches = by_deaths[key]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            print(f"  {key:>8} {len(matches):>4} {wr:>5.1f}% {eff:>8.3f}")


def generate_advice(all_data, ms_data):
    """総合アドバイスを生成"""
    advices = []

    # 被撃墜分析
    deaths_2plus = [d for d in all_data if d["deaths"] >= 2]
    if deaths_2plus:
        rate = len(deaths_2plus) / len(all_data) * 100
        wr = win_rate(deaths_2plus)
        advices.append(
            f"被撃墜2回以上の試合が全体の{rate:.0f}%あり、その勝率は{wr:.0f}%です。"
            f"2落ちを減らすことが勝率改善の最大のポイントです。"
        )

    # ダメ効率が1.0未満の機体
    for ms_name, data in ms_data.items():
        if len(data) < 3:
            continue
        eff = dmg_efficiency(data)
        if eff < 1.0:
            advices.append(
                f"【{ms_name}】のダメージ効率は{eff:.3f}で1.0未満です。"
                f"被ダメージが与ダメージを上回っており、立ち回りの改善が必要です。"
            )

    # 時間帯
    hourly = defaultdict(list)
    for d in all_data:
        hourly[d["datetime"].hour].append(d)

    bad_hours = []
    good_hours = []
    for hour, matches in hourly.items():
        if len(matches) >= 5:
            wr = win_rate(matches)
            if wr <= 40:
                bad_hours.append((hour, wr))
            elif wr >= 70:
                good_hours.append((hour, wr))

    if bad_hours:
        hours_str = "、".join(f"{h}時台({wr:.0f}%)" for h, wr in bad_hours)
        advices.append(f"勝率が低い時間帯: {hours_str}。この時間帯を避けるか、意識的にプレイしましょう。")

    if good_hours:
        hours_str = "、".join(f"{h}時台({wr:.0f}%)" for h, wr in good_hours)
        advices.append(f"勝率が高い時間帯: {hours_str}。調子の良い時間帯を活用しましょう。")

    # 苦手機体
    for ms_name, data in ms_data.items():
        if len(data) < 3:
            continue
        enemy_stats = defaultdict(list)
        for d in data:
            for ems in d["enemy_ms"]:
                enemy_stats[ems].append(d)

        weak_enemies = []
        for ems, matches in enemy_stats.items():
            if len(matches) >= 3 and win_rate(matches) <= 30:
                weak_enemies.append(f"{ems}({win_rate(matches):.0f}%)")

        if weak_enemies:
            advices.append(
                f"【{ms_name}】の苦手機体: {', '.join(weak_enemies)}。"
                f"対策を練るか、別の機体での対応を検討しましょう。"
            )

    # 平日vs土日
    weekday_data = [d for d in all_data if d["datetime"].weekday() < 5]
    weekend_data = [d for d in all_data if d["datetime"].weekday() >= 5]
    if weekday_data and weekend_data:
        wd_wr = win_rate(weekday_data)
        we_wr = win_rate(weekend_data)
        diff = abs(wd_wr - we_wr)
        if diff >= 10:
            better = "平日" if wd_wr > we_wr else "土日"
            worse = "土日" if wd_wr > we_wr else "平日"
            advices.append(
                f"{better}の勝率({max(wd_wr, we_wr):.0f}%)が{worse}({min(wd_wr, we_wr):.0f}%)より{diff:.0f}ポイント高いです。"
            )

    return advices


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <csv_path>")
        sys.exit(1)

    csv_path = sys.argv[1]
    matches = load_csv(csv_path)

    if not matches:
        print("試合データが見つかりませんでした。")
        sys.exit(1)

    # プレイヤー名を自動検出
    player_name = detect_player_name(matches)
    print(f"プレイヤー名を自動検出: 「{player_name}」")

    # 全試合データを変換
    all_data = []
    for m in matches:
        d = get_my_data(m)
        all_data.append(d)

    if not all_data:
        print("分析対象の試合データがありません。")
        sys.exit(1)

    # 機体別にグループ化（3戦未満の機体は「その他」にまとめない、個別分析をスキップ）
    ms_data = defaultdict(list)
    for d in all_data:
        ms_data[d["ms"]].append(d)

    # ========== レポート出力 ==========
    print_header(f"EXVS2IB 戦績分析レポート - 「{player_name}」")

    # 1. 全体基本スタッツ
    print_header("全体スタッツ", 2)
    analyze_basic_stats(all_data)

    print_header("勝ち/負け時のダメージ傾向", 2)
    analyze_win_loss_pattern(all_data)

    # 2. 機体別分析（試合数が多い順、3戦以上）
    for ms_name in sorted(ms_data.keys(), key=lambda x: -len(ms_data[x])):
        data = ms_data[ms_name]
        if len(data) < 3:
            continue

        print_header(f"機体別分析: {ms_name} ({len(data)}戦)")

        print_header("基本スタッツ", 2)
        analyze_basic_stats(data)

        print_header("勝ち/負け時のダメージ傾向", 2)
        analyze_win_loss_pattern(data)

        print_header("敵機体との相性", 2)
        analyze_enemy_matchup(data)

        print_header("相方機体との相性", 2)
        analyze_partner(data)

    # 3. 被撃墜数と勝率
    print_header("被撃墜数と勝率の関係")
    analyze_deaths_impact(all_data)

    # 4. 時間帯別
    print_header("時間帯別の勝率")
    analyze_time_of_day(all_data)

    # 5. 曜日別
    print_header("曜日別の勝率（平日 vs 土日）")
    analyze_day_of_week(all_data)

    # 6. 日別推移
    print_header("日別勝率推移")
    analyze_daily_trend(all_data)

    # 7. シーズン分析
    print_header("シーズン別分析")
    analyze_season(all_data)

    # 8. 総合アドバイス
    print_header("総合アドバイス")
    advices = generate_advice(all_data, ms_data)
    for i, advice in enumerate(advices, 1):
        print(f"  {i}. {advice}")


if __name__ == "__main__":
    main()
