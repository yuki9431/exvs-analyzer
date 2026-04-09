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
    p2 = match[1]
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
        "partner_name": p2["プレイヤー名"].strip(),
        "partner_ms": p2["機体名"].strip() or "(不明)",
        "partner_score": int(p2["スコア"]),
        "partner_kills": int(p2["撃墜数"]),
        "partner_deaths": int(p2["被撃墜数"]),
        "partner_dmg_given": int(p2["与ダメージ"]),
        "partner_dmg_taken": int(p2["被ダメージ"]),
        "enemy_ms": [
            match[2]["機体名"].strip() or "(不明)",
            match[3]["機体名"].strip() or "(不明)",
        ],
    }


def detect_fixed_partners(all_data, min_streak=3):
    """連続で同じ相方と組んでいる区間を検出し、固定相方の試合を返す。
    時系列順で連続3戦以上同じ相方名ならその区間を固定とみなす。
    """
    sorted_data = sorted(all_data, key=lambda d: d["datetime"])

    fixed_matches = defaultdict(list)
    streak = [sorted_data[0]]

    for i in range(1, len(sorted_data)):
        if sorted_data[i]["partner_name"] == streak[-1]["partner_name"]:
            streak.append(sorted_data[i])
        else:
            if len(streak) >= min_streak:
                name = streak[0]["partner_name"]
                fixed_matches[name].extend(streak)
            streak = [sorted_data[i]]

    # 最後のストリーク
    if len(streak) >= min_streak:
        name = streak[0]["partner_name"]
        fixed_matches[name].extend(streak)

    return fixed_matches


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
    eff = dmg_efficiency(data_list)

    lines = [
        "| 項目 | 値 |",
        "|------|-----|",
        f"| 試合数 | {n}戦 ({w}勝{l}敗) |",
        f"| **勝率** | **{wr:.1f}%** |",
        f"| 平均与ダメージ | {avg([d['dmg_given'] for d in data_list]):.0f} |",
        f"| 平均被ダメージ | {avg([d['dmg_taken'] for d in data_list]):.0f} |",
        f"| **ダメージ効率** | **{eff:.3f}** |",
        f"| 平均撃墜 | {avg([d['kills'] for d in data_list]):.2f} |",
        f"| 平均被撃墜 | {avg([d['deaths'] for d in data_list]):.2f} |",
        f"| K/D比 | {kd:.2f} |",
        f"| 平均EXダメージ | {avg([d['ex_dmg'] for d in data_list]):.0f} |",
        "",
    ]

    # セクション別アドバイス
    tips = []
    if eff < 1.0:
        tips.append(f"ダメージ効率が{eff:.3f}で1.0未満です。被ダメが与ダメを上回っており、被弾を減らす立ち回りが必要です。")
    elif eff >= 1.2:
        tips.append(f"ダメージ効率{eff:.3f}は優秀です。この調子を維持しましょう。")
    if kd < 1.0:
        tips.append(f"K/D比が{kd:.2f}で1.0未満です。撃墜数を増やすか、被撃墜を減らすことを意識しましょう。")
    if tips:
        lines.append("> **💡 アドバイス:** " + "<br>".join(tips))

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
    lines.append("")

    # セクション別アドバイス
    w_deaths = avg([d["deaths"] for d in wins]) if wins else 0
    l_deaths = avg([d["deaths"] for d in losses]) if losses else 0
    l_taken = avg([d["dmg_taken"] for d in losses]) if losses else 0
    tips = []
    if l_deaths >= 1.5:
        tips.append(f"負け試合の平均被撃墜が{l_deaths:.1f}と高いです。耐久管理を意識しましょう。")
    if l_taken >= 1100:
        tips.append(f"負け試合の被ダメージが平均{l_taken:.0f}と高いです。無駄な被弾を減らすことが改善の鍵です。")
    if tips:
        lines.append("> **💡 アドバイス:** " + "<br>".join(tips))

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

    strong = sorted([r for r in results if r[2] >= 60], key=lambda x: -x[1])
    weak = sorted([r for r in results if r[2] <= 40], key=lambda x: -x[1])
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

    # セクション別アドバイス
    tips = []
    if weak:
        high_dmg_taken = [r for r in weak if r[5] >= 1200]
        if high_dmg_taken:
            names = "、".join(r[0] for r in high_dmg_taken[:3])
            tips.append(f"{names} 戦では被ダメが特に多いです。距離管理を見直しましょう。")
        low_dmg_given = [r for r in weak if r[4] <= 900]
        if low_dmg_given:
            names = "、".join(r[0] for r in low_dmg_given[:3])
            tips.append(f"{names} 戦では与ダメが低いです。攻撃の手数や当て方を工夫しましょう。")
    if tips:
        lines.append("")
        lines.append("> **💡 アドバイス:** " + "<br>".join(tips))

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

    results.sort(key=lambda x: -x[1])
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

    d2 = len(by_deaths.get("2回", [])) + len(by_deaths.get("3回以上", []))
    total = len(data_list)
    lines.append("")
    lines.append(f"> **💡 アドバイス:** 被撃墜2回以上の試合は{d2}/{total}戦({d2/total*100:.0f}%)。1落ち以内に抑えれば勝率80%超が見込めます。耐久管理が最重要課題です。")

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

    good = [h for h, m in hourly.items() if len(m) >= 5 and win_rate(m) >= 70]
    bad = [h for h, m in hourly.items() if len(m) >= 5 and win_rate(m) <= 40]
    tips = []
    if good:
        tips.append(f"{'、'.join(f'{h}時台' for h in sorted(good))}が好調です。")
    if bad:
        tips.append(f"{'、'.join(f'{h}時台' for h in sorted(bad))}は不調です。強い相手が多い時間帯か、疲労の影響かもしれません。")
    if tips:
        lines.append("")
        lines.append("> **💡 アドバイス:** " + " ".join(tips))

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

    wd_wr = win_rate(weekday_data) if weekday_data else 0
    we_wr = win_rate(weekend_data) if weekend_data else 0
    diff = abs(wd_wr - we_wr)
    if diff >= 10:
        better = "平日" if wd_wr > we_wr else "土日"
        worse = "土日" if wd_wr > we_wr else "平日"
        lines.append("")
        lines.append(f"> **💡 アドバイス:** {better}の方が{worse}より勝率が{diff:.0f}ポイント高いです。{worse}は対戦相手の質が変わる可能性があります。")
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

    # 連敗ストリーク検出
    sorted_data = sorted(data_list, key=lambda d: d["datetime"])
    max_lose_streak = 0
    current_streak = 0
    for d in sorted_data:
        if not d["win"]:
            current_streak += 1
            max_lose_streak = max(max_lose_streak, current_streak)
        else:
            current_streak = 0

    tips = []
    bad_days = [ds for ds in sorted(daily.keys()) if win_rate(daily[ds]) <= 40 and len(daily[ds]) >= 5]
    if bad_days:
        tips.append(f"勝率40%以下の日: {', '.join(bad_days)}。不調時は早めに切り上げましょう。")
    if max_lose_streak >= 4:
        tips.append(f"最大{max_lose_streak}連敗の記録があります。3連敗したら休憩を挟むことを推奨します。")
    if tips:
        lines.append("")
        lines.append("> **💡 アドバイス:** " + "<br>".join(tips))

    return "\n".join(lines)


def md_fixed_partners(all_data):
    """固定相方（連続3戦以上）の分析"""
    fixed = detect_fixed_partners(all_data)

    if not fixed:
        return "固定相方（連続3戦以上）は検出されませんでした。"

    lines = []
    for partner_name in sorted(fixed.keys(), key=lambda x: -len(fixed[x])):
        data = fixed[partner_name]
        n = len(data)
        w, l = wins_losses(data)
        wr = w / n * 100

        # 自分のスタッツ
        my_eff = dmg_efficiency(data)
        my_avg_given = avg([d["dmg_given"] for d in data])
        my_avg_taken = avg([d["dmg_taken"] for d in data])

        # 相方のスタッツ
        p_avg_given = avg([d["partner_dmg_given"] for d in data])
        p_avg_taken = avg([d["partner_dmg_taken"] for d in data])
        p_total_given = sum(d["partner_dmg_given"] for d in data)
        p_total_taken = sum(d["partner_dmg_taken"] for d in data)
        p_eff = p_total_given / p_total_taken if p_total_taken > 0 else 0
        p_avg_kills = avg([d["partner_kills"] for d in data])
        p_avg_deaths = avg([d["partner_deaths"] for d in data])

        lines.append(f"### {partner_name} ({n}戦)\n")
        lines.append(f"**チーム成績: {w}勝{l}敗 (勝率{wr:.1f}%)**\n")

        lines.append("| 項目 | 自分 | 相方 |")
        lines.append("|------|------|------|")
        lines.append(f"| 平均与ダメージ | {my_avg_given:.0f} | {p_avg_given:.0f} |")
        lines.append(f"| 平均被ダメージ | {my_avg_taken:.0f} | {p_avg_taken:.0f} |")
        lines.append(f"| ダメージ効率 | {my_eff:.3f} | {p_eff:.3f} |")
        lines.append(f"| 平均撃墜 | {avg([d['kills'] for d in data]):.2f} | {p_avg_kills:.2f} |")
        lines.append(f"| 平均被撃墜 | {avg([d['deaths'] for d in data]):.2f} | {p_avg_deaths:.2f} |")

        # 相方の使用機体別勝率
        partner_ms_stats = defaultdict(list)
        for d in data:
            partner_ms_stats[d["partner_ms"]].append(d)

        if len(partner_ms_stats) > 1 or any(len(v) >= 2 for v in partner_ms_stats.values()):
            lines.append("\n**相方の使用機体別:**\n")
            lines.append("| 機体 | 試合 | 勝率 | 相方ダメ効率 |")
            lines.append("|------|------|------|-------------|")
            for ms in sorted(partner_ms_stats.keys(), key=lambda x: -len(partner_ms_stats[x])):
                ms_data = partner_ms_stats[ms]
                ms_wr = win_rate(ms_data)
                ms_p_given = sum(d["partner_dmg_given"] for d in ms_data)
                ms_p_taken = sum(d["partner_dmg_taken"] for d in ms_data)
                ms_p_eff = ms_p_given / ms_p_taken if ms_p_taken > 0 else 0
                lines.append(f"| {ms} | {len(ms_data)} | {ms_wr:.0f}% | {ms_p_eff:.3f} |")

        # 相方ごとのアドバイス
        tips = []
        if p_eff < 0.8:
            tips.append(f"相方のダメ効率が{p_eff:.3f}と低めです。相方が狙われやすい展開になっている可能性があります。カットやラインを意識しましょう。")
        if wr < 45 and n >= 5:
            tips.append(f"勝率が{wr:.0f}%と低調です。連携や機体の組み合わせを見直してみましょう。")
        if n >= 5:
            if wr >= 90:
                tips.append(f"驚異的！勝率{wr:.0f}%は全国大会優勝レベルです。何も言うことがありません！")
            elif wr >= 80:
                tips.append(f"圧巻！勝率{wr:.0f}%は全国大会上位クラスです。勝ちパターンを分析して再現性を高めましょう。")
            elif wr >= 70:
                tips.append(f"素晴らしい相性です！勝率{wr:.0f}%は上位プレイヤーに匹敵する勝率です。この相方との連携を軸に、苦手な機体への対策を詰めていきましょう。")
            elif wr >= 60:
                tips.append(f"好調です！勝率{wr:.0f}%、安定した連携ができています。さらに勝率を伸ばすために相方との役割分担を意識してみましょう。")
        if tips:
            lines.append(f"> **💡 アドバイス:** " + "<br>".join(tips))

        lines.append("")

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

        first_half = season_half[season_name].get("前半", [])
        second_half = season_half[season_name].get("後半", [])
        if first_half:
            lines.append(f"- 前半: {len(first_half)}戦 勝率{win_rate(first_half):.1f}% ダメ効率{dmg_efficiency(first_half):.3f}")
        if second_half:
            lines.append(f"- 後半: {len(second_half)}戦 勝率{win_rate(second_half):.1f}% ダメ効率{dmg_efficiency(second_half):.3f}")

        if first_half and second_half:
            f_wr = win_rate(first_half)
            s_wr = win_rate(second_half)
            diff = s_wr - f_wr
            if abs(diff) >= 5:
                if diff > 0:
                    lines.append(f"\n> **💡 アドバイス:** 後半の方が勝率が{diff:.0f}ポイント高く、シーズンが進むにつれて安定しています。")
                else:
                    lines.append(f"\n> **💡 アドバイス:** 前半の方が勝率が{-diff:.0f}ポイント高いです。後半は対戦相手のレベルが上がっている可能性があります。")
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

    # 固定相方ごとの勝率差
    fixed = detect_fixed_partners(all_data)
    if len(fixed) >= 2:
        partner_wrs = [(name, win_rate(data), len(data)) for name, data in fixed.items() if len(data) >= 5]
        if len(partner_wrs) >= 2:
            partner_wrs.sort(key=lambda x: -x[1])
            best = partner_wrs[0]
            worst = partner_wrs[-1]
            if best[1] - worst[1] >= 15:
                advices.append(
                    f"固定相方の勝率差が大きいです。{best[0]}({best[1]:.0f}%)と{worst[0]}({worst[1]:.0f}%)で{best[1]-worst[1]:.0f}ポイント差。"
                    f"相方ごとに戦い方を変えるか、相性の良い相方との試合を増やしましょう。"
                )

    # 連敗ストリーク
    sorted_data = sorted(all_data, key=lambda d: d["datetime"])
    max_lose_streak = 0
    current_streak = 0
    for d in sorted_data:
        if not d["win"]:
            current_streak += 1
            max_lose_streak = max(max_lose_streak, current_streak)
        else:
            current_streak = 0
    if max_lose_streak >= 4:
        advices.append(
            f"最大{max_lose_streak}連敗の記録があります。3連敗したら休憩を挟みましょう。メンタル管理も勝率に直結します。"
        )

    # シーズン前半/後半
    season_data = defaultdict(list)
    season_half = defaultdict(lambda: defaultdict(list))
    for d in all_data:
        s = get_season(d["datetime"])
        h = get_season_half(d["datetime"])
        season_data[s].append(d)
        season_half[s][h].append(d)

    for season_name in season_data:
        first = season_half[season_name].get("前半", [])
        second = season_half[season_name].get("後半", [])
        if first and second:
            f_wr = win_rate(first)
            s_wr = win_rate(second)
            diff = s_wr - f_wr
            if abs(diff) >= 10:
                if diff > 0:
                    advices.append(f"{season_name}: 後半の勝率が前半より{diff:.0f}ポイント高く、シーズン後半に安定する傾向があります。")
                else:
                    advices.append(f"{season_name}: 前半の勝率が後半より{-diff:.0f}ポイント高いです。後半は対戦環境が厳しくなっている可能性があります。")

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

    # 目次
    def toc_link(label, heading):
        anchor = heading.replace(" ", "-")
        return f"[{label}](#{anchor})"

    ms_names_for_toc = [ms for ms in sorted(ms_data.keys(), key=lambda x: -len(ms_data[x])) if len(ms_data[ms]) >= 3]
    toc = ["<details open><summary><strong>目次</strong></summary>\n"]
    toc.append(f"1. {toc_link('基本データ', '基本データ')}")
    n = 2
    for i, ms_name in enumerate(ms_names_for_toc):
        ms_count = len(ms_data[ms_name])
        heading = f"機体別分析:-{ms_name}-({ms_count}戦)"
        toc.append(f"{n+i}. {toc_link(ms_name + ' (' + str(ms_count) + '戦)', heading)}")
        toc.append(f"   - {toc_link('基本データ', '基本データ（' + ms_name + '）')}")
        toc.append(f"   - {toc_link('敵機体との相性', '敵機体との相性（' + ms_name + '）')}")
        toc.append(f"   - {toc_link('相方機体との相性', '相方機体との相性（' + ms_name + '）')}")
    n += len(ms_names_for_toc)
    toc.append(f"{n}. {toc_link('固定相方分析', '固定相方分析（連続3戦以上）')}")
    toc.append(f"{n+1}. {toc_link('被撃墜数と勝率', '被撃墜数と勝率の関係')}")
    toc.append(f"{n+2}. {toc_link('時間帯別', '時間帯別の勝率')}")
    toc.append(f"{n+3}. {toc_link('曜日別', '曜日別の勝率（平日-vs-土日）')}")
    toc.append(f"{n+4}. {toc_link('日別推移', '日別勝率推移')}")
    toc.append(f"{n+5}. {toc_link('シーズン別', 'シーズン別分析')}")
    toc.append(f"{n+6}. {toc_link('総合アドバイス', '総合アドバイス')}")
    toc.append("\n</details>")
    report.append("\n".join(toc))

    # 基本データ
    report.append("\n\n---\n\n## 基本データ\n")
    report.append(md_basic_stats(all_data))
    report.append("\n### 勝ち/負け時のダメージ傾向\n")
    report.append(md_win_loss_pattern(all_data))

    # 機体別分析
    for ms_name in ms_names_for_toc:
        data = ms_data[ms_name]

        report.append(f"\n---\n\n## 機体別分析: {ms_name} ({len(data)}戦)\n")
        report.append(f"### 基本データ（{ms_name}）\n")
        report.append(md_basic_stats(data))
        report.append(f"\n### 勝ち/負け時のダメージ傾向（{ms_name}）\n")
        report.append(md_win_loss_pattern(data))
        report.append(f"\n### 敵機体との相性（{ms_name}）\n")
        report.append(md_enemy_matchup(data))
        report.append(f"\n### 相方機体との相性（{ms_name}）\n")
        report.append(md_partner(data))

    # 固定相方分析
    report.append("\n---\n\n## 固定相方分析（連続3戦以上）\n")
    report.append(md_fixed_partners(all_data))

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
