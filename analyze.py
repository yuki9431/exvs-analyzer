#!/usr/bin/env python3
"""
EXVS2IB 戦績分析スクリプト
CSVファイルを読み込み、マークダウン形式の分析レポートを出力する。
プレイヤー名はCSVのプレイヤーNo.1から自動取得する。
"""

import csv
import os
import sys
from collections import defaultdict
from datetime import datetime


def get_season(dt):
    """日付からシーズン名を返す。偶数月開始の2ヶ月区切り。
    12-1月, 2-3月, 4-5月, 6-7月, 8-9月, 10-11月
    """
    m = dt.month
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

    if start_month == 12:
        return f"{year}年{start_month}月-{end_year}年{end_month}月"
    return f"{year}年{start_month}-{end_month}月"


def get_season_half(dt):
    """シーズン内で前半/後半を返す。開始月=前半、翌月=後半。"""
    m = dt.month
    if m == 12 or m % 2 == 0:
        return "前半"
    return "後半"


def load_csv(path):
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
    names = defaultdict(int)
    for m in matches:
        name = m[0]["プレイヤー名"].strip()
        if name:
            names[name] += 1
    return max(names, key=names.get) if names else ""


def get_player_ms(match):
    ms = match[0]["機体名"].strip()
    return ms if ms else "(不明)"


def get_my_data(match):
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


def wins_losses(data_list):
    w = sum(1 for d in data_list if d["win"])
    return w, len(data_list) - w


# ========== マークダウン出力関数 ==========


def md_basic_stats(data_list):
    n = len(data_list)
    w, l = wins_losses(data_list)
    wr = w / n * 100 if n > 0 else 0
    total_kills = sum(d["kills"] for d in data_list)
    total_deaths = sum(d["deaths"] for d in data_list)
    kd = total_kills / total_deaths if total_deaths > 0 else 0

    lines = [
        "| 項目 | 値 |",
        "|------|-----|",
        f"| 試合数 | {n}戦 ({w}勝{l}敗) |",
        f"| **勝率** | **{wr:.1f}%** |",
        f"| 平均与ダメージ | {avg([d['dmg_given'] for d in data_list]):.0f} |",
        f"| 平均被ダメージ | {avg([d['dmg_taken'] for d in data_list]):.0f} |",
        f"| **ダメージ効率** | **{dmg_efficiency(data_list):.3f}** |",
        f"| 平均撃墜 | {avg([d['kills'] for d in data_list]):.2f} |",
        f"| 平均被撃墜 | {avg([d['deaths'] for d in data_list]):.2f} |",
        f"| K/D比 | {kd:.2f} |",
        f"| 平均EXダメージ | {avg([d['ex_dmg'] for d in data_list]):.0f} |",
    ]
    return "\n".join(lines)


def md_win_loss_pattern(data_list):
    wins = [d for d in data_list if d["win"]]
    losses = [d for d in data_list if not d["win"]]

    lines = [
        "| 項目 | 勝ち | 負け | 差 |",
        "|------|------|------|-----|",
    ]
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
        lines.append(f"| {label} | {w_avg:.1f} | {l_avg:.1f} | {sign}{diff:.1f} |")

    w_eff = dmg_efficiency(wins) if wins else 0
    l_eff = dmg_efficiency(losses) if losses else 0
    lines.append(f"| **ダメ効率** | **{w_eff:.3f}** | **{l_eff:.3f}** | {w_eff - l_eff:+.3f} |")
    return "\n".join(lines)


def md_enemy_matchup(data_list, min_matches=3):
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

    header_row = "| 機体名 | 試合 | 勝率 | ダメ効率 | 与ダメ | 被ダメ |"
    header_sep = "|--------|------|------|----------|--------|--------|"
    lines = []

    if weak:
        lines.append("**苦手な相手 (勝率40%以下):**\n")
        lines.append(header_row)
        lines.append(header_sep)
        for ms, n, wr, eff, gv, tk in weak:
            lines.append(f"| {ms} | {n} | {wr:.0f}% | {eff:.3f} | {gv:.0f} | {tk:.0f} |")
        lines.append("")

    if strong:
        lines.append("**得意な相手 (勝率60%以上):**\n")
        lines.append(header_row)
        lines.append(header_sep)
        for ms, n, wr, eff, gv, tk in strong:
            lines.append(f"| {ms} | {n} | {wr:.0f}% | {eff:.3f} | {gv:.0f} | {tk:.0f} |")
        lines.append("")

    if even:
        lines.append("**五分の相手 (勝率41-59%):**\n")
        lines.append(header_row)
        lines.append(header_sep)
        for ms, n, wr, eff, gv, tk in even:
            lines.append(f"| {ms} | {n} | {wr:.0f}% | {eff:.3f} | {gv:.0f} | {tk:.0f} |")

    return "\n".join(lines)


def md_partner(data_list, min_matches=3):
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
    lines = [
        "| 相方機体 | 試合 | 勝率 | ダメ効率 |",
        "|----------|------|------|----------|",
    ]
    for ms, n, wr, eff in results:
        lines.append(f"| {ms} | {n} | {wr:.0f}% | {eff:.3f} |")
    return "\n".join(lines)


def md_deaths_impact(data_list):
    by_deaths = defaultdict(list)
    for d in data_list:
        deaths = d["deaths"]
        key = f"{deaths}回" if deaths <= 2 else "3回以上"
        by_deaths[key].append(d)

    lines = [
        "| 被撃墜 | 試合 | 勝率 | ダメ効率 |",
        "|--------|------|------|----------|",
    ]
    for key in ["0回", "1回", "2回", "3回以上"]:
        if key in by_deaths:
            matches = by_deaths[key]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            lines.append(f"| {key} | {len(matches)} | **{wr:.1f}%** | {eff:.3f} |")
    return "\n".join(lines)


def md_time_of_day(data_list):
    hourly = defaultdict(list)
    for d in data_list:
        hourly[d["datetime"].hour].append(d)

    lines = [
        "| 時間帯 | 試合 | 勝率 | ダメ効率 | |",
        "|--------|------|------|----------|-|",
    ]
    for hour in sorted(hourly.keys()):
        matches = hourly[hour]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        mark = "★" if wr >= 70 else "▼" if wr <= 40 else ""
        lines.append(f"| {hour}時台 | {len(matches)} | {wr:.1f}% | {eff:.3f} | {mark} |")
    return "\n".join(lines)


def md_day_of_week(data_list):
    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    daily = defaultdict(list)
    for d in data_list:
        daily[d["datetime"].weekday()].append(d)

    weekday_data = [d for d in data_list if d["datetime"].weekday() < 5]
    weekend_data = [d for d in data_list if d["datetime"].weekday() >= 5]

    lines = [
        f"- **平日**: {len(weekday_data)}戦 勝率{win_rate(weekday_data):.1f}% ダメ効率{dmg_efficiency(weekday_data):.3f}",
        f"- **土日**: {len(weekend_data)}戦 勝率{win_rate(weekend_data):.1f}% ダメ効率{dmg_efficiency(weekend_data):.3f}",
        "",
        "| 曜日 | 試合 | 勝率 | ダメ効率 |",
        "|------|------|------|----------|",
    ]
    for dow in range(7):
        if dow in daily:
            matches = daily[dow]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            lines.append(f"| {DOW_NAMES[dow]} | {len(matches)} | {wr:.1f}% | {eff:.3f} |")
    return "\n".join(lines)


def md_daily_trend(data_list):
    daily = defaultdict(list)
    for d in data_list:
        date_str = d["datetime"].strftime("%m/%d")
        daily[date_str].append(d)

    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    lines = [
        "| 日付 | 曜日 | 試合 | 勝率 | ダメ効率 | |",
        "|------|------|------|------|----------|-|",
    ]
    for date_str in sorted(daily.keys()):
        matches = daily[date_str]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        dow = matches[0]["datetime"].weekday()
        mark = "★" if wr >= 70 else "▼" if wr <= 45 else ""
        lines.append(f"| {date_str} | {DOW_NAMES[dow]} | {len(matches)} | {wr:.1f}% | {eff:.3f} | {mark} |")
    return "\n".join(lines)


def md_season(data_list):
    season_data = defaultdict(list)
    season_half = defaultdict(lambda: defaultdict(list))

    for d in data_list:
        s = get_season(d["datetime"])
        h = get_season_half(d["datetime"])
        season_data[s].append(d)
        season_half[s][h].append(d)

    lines = []
    for season_name in sorted(season_data.keys()):
        data = season_data[season_name]
        lines.append(f"**{season_name}**\n")
        lines.append(f"- 全体: {len(data)}戦 勝率{win_rate(data):.1f}% ダメ効率{dmg_efficiency(data):.3f}")
        for half_name in ["前半", "後半"]:
            hdata = season_half[season_name][half_name]
            if hdata:
                lines.append(f"- {half_name}: {len(hdata)}戦 勝率{win_rate(hdata):.1f}% ダメ効率{dmg_efficiency(hdata):.3f}")
        lines.append("")
    return "\n".join(lines)


def md_advice(all_data, ms_data):
    advices = []

    deaths_2plus = [d for d in all_data if d["deaths"] >= 2]
    if deaths_2plus:
        rate = len(deaths_2plus) / len(all_data) * 100
        wr = win_rate(deaths_2plus)
        advices.append(
            f"被撃墜2回以上の試合が全体の{rate:.0f}%あり、その勝率は{wr:.0f}%です。"
            f"**2落ちを減らすことが勝率改善の最大のポイント**です。"
        )

    for ms_name, data in ms_data.items():
        if len(data) < 3:
            continue
        eff = dmg_efficiency(data)
        if eff < 1.0:
            advices.append(
                f"**{ms_name}** のダメージ効率は{eff:.3f}で1.0未満です。"
                f"被ダメージが与ダメージを上回っており、立ち回りの改善が必要です。"
            )

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
        advices.append(f"勝率が高い時間帯: {hours_str}。この時間帯を活用しましょう。")

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
                f"**{ms_name}** の苦手機体: {', '.join(weak_enemies)}。"
                f"対策を練るか、別の機体での対応を検討しましょう。"
            )

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

    lines = []
    for i, a in enumerate(advices, 1):
        lines.append(f"{i}. {a}")
    return "\n".join(lines)


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <csv_path>")
        sys.exit(1)

    csv_path = sys.argv[1]
    matches = load_csv(csv_path)

    if not matches:
        print("試合データが見つかりませんでした。")
        sys.exit(1)

    player_name = detect_player_name(matches)

    all_data = [get_my_data(m) for m in matches]

    ms_data = defaultdict(list)
    for d in all_data:
        ms_data[d["ms"]].append(d)

    # ========== レポート生成 ==========
    report = []
    report.append(f"# EXVS2IB 戦績分析レポート - 「{player_name}」\n")

    # 全体スタッツ
    report.append("## 全体スタッツ\n")
    report.append(md_basic_stats(all_data))
    report.append("\n### 勝ち/負け時のダメージ傾向\n")
    report.append(md_win_loss_pattern(all_data))

    # 機体別分析
    for ms_name in sorted(ms_data.keys(), key=lambda x: -len(ms_data[x])):
        data = ms_data[ms_name]
        if len(data) < 3:
            continue

        report.append(f"\n---\n\n## 機体別分析: {ms_name} ({len(data)}戦)\n")
        report.append("### 基本スタッツ\n")
        report.append(md_basic_stats(data))
        report.append("\n### 勝ち/負け時のダメージ傾向\n")
        report.append(md_win_loss_pattern(data))
        report.append("\n### 敵機体との相性\n")
        report.append(md_enemy_matchup(data))
        report.append("\n### 相方機体との相性\n")
        report.append(md_partner(data))

    # 被撃墜数と勝率
    report.append("\n---\n\n## 被撃墜数と勝率の関係\n")
    report.append(md_deaths_impact(all_data))

    # 時間帯別
    report.append("\n## 時間帯別の勝率\n")
    report.append(md_time_of_day(all_data))

    # 曜日別
    report.append("\n## 曜日別の勝率（平日 vs 土日）\n")
    report.append(md_day_of_week(all_data))

    # 日別推移
    report.append("\n## 日別勝率推移\n")
    report.append(md_daily_trend(all_data))

    # シーズン分析
    report.append("\n## シーズン別分析\n")
    report.append(md_season(all_data))

    # 総合アドバイス
    report.append("\n---\n\n## 総合アドバイス\n")
    report.append(md_advice(all_data, ms_data))

    # ファイル出力
    output_path = os.path.join(os.path.dirname(csv_path), "report.md")
    content = "\n".join(report) + "\n"

    with open(output_path, "w", encoding="utf-8") as f:
        f.write(content)

    print(f"分析レポートを出力しました: {output_path}")


if __name__ == "__main__":
    main()
