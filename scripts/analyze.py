#!/usr/bin/env python3
"""
EXVS2IB 戦績分析スクリプト
CSVファイルを読み込み、マークダウン形式の分析レポートを出力する。
プレイヤー名はCSVのプレイヤーNo.1から自動取得する。
"""

import csv
import json
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


def load_ms_cost_map(csv_path):
    """ms_list.jsonから画像URL→コストのマップを生成する。
    クエリパラメータを除去してマッチさせる。
    """
    ms_list_path = os.path.join(os.path.dirname(os.path.dirname(csv_path)), "data", "ms_list.json")
    if not os.path.exists(ms_list_path):
        # スクリプト配置場所からの相対パスでもフォールバック
        ms_list_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "data", "ms_list.json")
    if not os.path.exists(ms_list_path):
        return {}
    with open(ms_list_path, "r", encoding="utf-8") as f:
        ms_list = json.load(f)
    cost_map = {}
    for ms in ms_list:
        url = ms.get("ImageURL", "")
        cost = ms.get("Cost", 0)
        if url and cost:
            # クエリパラメータを除去
            url = url.split("?")[0]
            cost_map[url] = cost
    return cost_map


def get_player_ms(match):
    ms = match[0]["機体名"].strip()
    return ms if ms else "(不明)"


# コスト別の「負けに直結する被撃墜数」（この回数で6000コスト以上消費＝負け確定）
COST_FATAL_DEATHS = {
    3000: 2,  # 3000×2=6000
    2500: 3,  # 2500×3=7500
    2000: 3,  # 2000×3=6000
    1500: 4,  # 1500×4=6000
}

# コスト帯の表示名
COST_LABEL = {
    3000: "3000コスト",
    2500: "2500コスト",
    2000: "2000コスト",
    1500: "1500コスト",
}


def get_my_data(match, cost_map=None):
    p = match[0]
    p2 = match[1]
    cost_map = cost_map or {}

    def lookup_cost(row):
        url = row.get("機体画像URL", "").strip().split("?")[0]
        return cost_map.get(url, 0)

    return {
        "win": p["勝利判定"] == "win",
        "ms": get_player_ms(match),
        "ms_cost": lookup_cost(p),
        "score": int(p["スコア"]),
        "kills": int(p["撃墜数"]),
        "deaths": int(p["被撃墜数"]),
        "dmg_given": int(p["与ダメージ"]),
        "dmg_taken": int(p["被ダメージ"]),
        "ex_dmg": int(p["EXダメージ"]),
        "datetime": datetime.strptime(p["試合日時"], "%Y-%m-%d %H:%M"),
        "partner_name": p2["プレイヤー名"].strip(),
        "partner_ms": p2["機体名"].strip() or "(不明)",
        "partner_cost": lookup_cost(p2),
        "partner_score": int(p2["スコア"]),
        "partner_kills": int(p2["撃墜数"]),
        "partner_deaths": int(p2["被撃墜数"]),
        "partner_dmg_given": int(p2["与ダメージ"]),
        "partner_dmg_taken": int(p2["被ダメージ"]),
        "enemy_ms": [
            match[2]["機体名"].strip() or "(不明)",
            match[3]["機体名"].strip() or "(不明)",
        ],
        "enemy_costs": [lookup_cost(match[2]), lookup_cost(match[3])],
    }


def detect_fixed_partners(all_data, min_streak=10):
    """連続で同じ相方と組んでいる区間を検出し、固定相方の試合を返す。
    時系列順で連続10戦以上同じ相方名ならその区間を固定とみなす。
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


def md_cost_pair(data_list, min_matches=3):
    """自機コスト+相方コストの組み合わせ別勝率"""
    pairs = defaultdict(list)
    for d in data_list:
        my_cost = d.get("ms_cost", 0)
        partner_cost = d.get("partner_cost", 0)
        if my_cost and partner_cost:
            key = f"{my_cost}+{partner_cost}"
            pairs[key].append(d)

    results = []
    for pair, matches in pairs.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            results.append((pair, len(matches), wr, eff))

    results.sort(key=lambda x: -x[1])
    lines = [
        "| コスト編成 | 試合 | 勝率 | 与被ダメ比 |",
        "|------------|------|------|----------|",
    ]
    for pair, n, wr, eff in results:
        lines.append(f"| {pair} | {n} | {wr:.1f}% | {eff:.3f} |")
    return "\n".join(lines)


def md_dmg_contribution(data_list, min_matches=3):
    """ダメージ貢献率（自分の与ダメ / チーム合計与ダメ）"""
    # 全体の貢献率
    contribs = []
    for d in data_list:
        team_total = d["dmg_given"] + d["partner_dmg_given"]
        if team_total > 0:
            contribs.append(d["dmg_given"] / team_total * 100)
    avg_contrib = sum(contribs) / len(contribs) if contribs else 0

    # 勝ち/負け別
    win_contribs = []
    lose_contribs = []
    for d in data_list:
        team_total = d["dmg_given"] + d["partner_dmg_given"]
        if team_total > 0:
            c = d["dmg_given"] / team_total * 100
            if d["win"]:
                win_contribs.append(c)
            else:
                lose_contribs.append(c)

    avg_win = sum(win_contribs) / len(win_contribs) if win_contribs else 0
    avg_lose = sum(lose_contribs) / len(lose_contribs) if lose_contribs else 0

    lines = [
        f"- 平均ダメージ貢献率: **{avg_contrib:.1f}%**（チーム与ダメに占める自分の割合）",
        f"- 勝ち試合: {avg_win:.1f}% / 負け試合: {avg_lose:.1f}%",
        "",
    ]

    # コスト帯別
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_LABEL:
            cost_groups[cost].append(d)

    if len(cost_groups) > 1 or any(len(v) >= min_matches for v in cost_groups.values()):
        lines.append("| コスト | 試合 | 貢献率 | 勝ち時 | 負け時 |")
        lines.append("|--------|------|--------|--------|--------|")
        for cost in sorted(cost_groups.keys(), reverse=True):
            data = cost_groups[cost]
            if len(data) < min_matches:
                continue
            c_all = []
            c_win = []
            c_lose = []
            for d in data:
                team_total = d["dmg_given"] + d["partner_dmg_given"]
                if team_total > 0:
                    c = d["dmg_given"] / team_total * 100
                    c_all.append(c)
                    if d["win"]:
                        c_win.append(c)
                    else:
                        c_lose.append(c)
            a = sum(c_all) / len(c_all) if c_all else 0
            w = sum(c_win) / len(c_win) if c_win else 0
            l = sum(c_lose) / len(c_lose) if c_lose else 0
            lines.append(f"| {COST_LABEL[cost]} | {len(data)} | {a:.1f}% | {w:.1f}% | {l:.1f}% |")

    return "\n".join(lines)


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
        f"| **与被ダメ比** | **{eff:.3f}** |",
        f"| 平均撃墜 | {avg([d['kills'] for d in data_list]):.2f} |",
        f"| 平均被撃墜 | {avg([d['deaths'] for d in data_list]):.2f} |",
        f"| K/D比 | {kd:.2f} |",
        f"| 平均EXダメージ | {avg([d['ex_dmg'] for d in data_list]):.0f} |",
        "",
    ]

    # セクション別アドバイス
    tips = []
    if eff < 1.0:
        tips.append(f"与被ダメ比が{eff:.3f}で1.0未満です。被ダメが与ダメを上回っており、被弾を減らす立ち回りが必要です。")
    elif eff >= 1.2:
        tips.append(f"与被ダメ比{eff:.3f}は優秀です。この調子を維持しましょう。")
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
    lines.append(f"| **与被ダメ比** | **{w_eff:.3f}** | **{l_eff:.3f}** | {w_eff - l_eff:+.3f} |")
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

    # コスト帯ごとの勝ちパターン分析
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_LABEL:
            cost_groups[cost].append(d)

    lines.append("\n### コスト帯別 勝ちパターン分析\n")
    for cost in sorted(cost_groups.keys(), reverse=True):
        data = cost_groups[cost]
        if len(data) < 3:
            continue
        label = COST_LABEL[cost]
        fatal = COST_FATAL_DEATHS[cost]
        c_wins = [d for d in data if d["win"]]
        c_losses = [d for d in data if not d["win"]]
        if not c_wins or not c_losses:
            continue

        lines.append(f"**{label}（{len(data)}戦 / 勝率{win_rate(data):.1f}%）**\n")
        lines.append("| 項目 | 勝ち | 負け | 差 |")
        lines.append("|------|------|------|-----|")

        for m_label, key in [("与ダメージ", "dmg_given"), ("被ダメージ", "dmg_taken"),
                             ("撃墜", "kills"), ("被撃墜", "deaths")]:
            w_v = avg([d[key] for d in c_wins])
            l_v = avg([d[key] for d in c_losses])
            diff = w_v - l_v
            sign = "+" if diff >= 0 else ""
            lines.append(f"| {m_label} | {w_v:.1f} | {l_v:.1f} | {sign}{diff:.1f} |")

        c_w_eff = dmg_efficiency(c_wins)
        c_l_eff = dmg_efficiency(c_losses)
        lines.append(f"| **与被ダメ比** | **{c_w_eff:.3f}** | **{c_l_eff:.3f}** | {c_w_eff - c_l_eff:+.3f} |")

        # 負け試合で負け確定ラインに達した割合
        fatal_losses = [d for d in c_losses if d["deaths"] >= fatal]
        if c_losses:
            fatal_rate = len(fatal_losses) / len(c_losses) * 100
            lines.append(f"\n負け試合のうち{fatal}落ち以上: {len(fatal_losses)}/{len(c_losses)}戦({fatal_rate:.0f}%)")

        lines.append("")

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

    header_row = "| 機体名 | 試合 | 勝率 | 与被ダメ比 | 与ダメ | 被ダメ |"
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
        "| 相方機体 | 試合 | 勝率 | 与被ダメ比 |",
        "|----------|------|------|----------|",
    ]
    for ms, n, wr, eff in results:
        lines.append(f"| {ms} | {n} | {wr:.0f}% | {eff:.3f} |")
    return "\n".join(lines)


def md_deaths_impact(data_list):
    # コスト帯別に分類
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_FATAL_DEATHS:
            cost_groups[cost].append(d)

    lines = []
    for cost in sorted(cost_groups.keys(), reverse=True):
        data = cost_groups[cost]
        fatal = COST_FATAL_DEATHS[cost]
        label = COST_LABEL[cost]
        lines.append(f"### {label}（{len(data)}戦）\n")
        lines.append(f"負け確定ライン: **{fatal}落ち**（{cost}×{fatal}={cost*fatal}コスト消費）\n")

        by_deaths = defaultdict(list)
        max_bucket = fatal + 1
        for d in data:
            deaths = d["deaths"]
            if deaths >= max_bucket:
                key = f"{max_bucket}回以上"
            else:
                key = f"{deaths}回"
            by_deaths[key].append(d)

        lines.append("| 被撃墜 | 試合 | 勝率 | 与被ダメ比 | 判定 |")
        lines.append("|--------|------|------|----------|------|")
        for i in range(max_bucket):
            key = f"{i}回"
            if key in by_deaths:
                matches = by_deaths[key]
                wr = win_rate(matches)
                eff = dmg_efficiency(matches)
                if i >= fatal:
                    mark = "⚠️ 負け確定"
                elif i == fatal - 1:
                    mark = "⚡ 危険"
                else:
                    mark = "✅ 安全"
                lines.append(f"| {key} | {len(matches)} | **{wr:.1f}%** | {eff:.3f} | {mark} |")
        over_key = f"{max_bucket}回以上"
        if over_key in by_deaths:
            matches = by_deaths[over_key]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            lines.append(f"| {over_key} | {len(matches)} | **{wr:.1f}%** | {eff:.3f} | ⚠️ 負け確定 |")

        # データに基づくアドバイス
        fatal_count = sum(len(by_deaths.get(f"{i}回", [])) for i in range(fatal, max_bucket))
        fatal_count += len(by_deaths.get(over_key, []))
        total = len(data)
        safe_data = []
        for i in range(fatal):
            safe_data.extend(by_deaths.get(f"{i}回", []))
        safe_wr = win_rate(safe_data) if safe_data else 0

        tips = []
        if fatal_count > 0 and total > 0:
            tips.append(f"負け確定({fatal}落ち以上)に達した試合は{fatal_count}/{total}戦({fatal_count/total*100:.0f}%)。")
        if safe_data:
            tips.append(f"{fatal-1}落ち以内に抑えた場合の勝率は**{safe_wr:.1f}%**。")
        if tips:
            lines.append("")
            lines.append("> **💡 アドバイス:** " + "".join(tips))
        lines.append("")

    return "\n".join(lines)


def md_time_of_day(data_list):
    hourly = defaultdict(list)
    for d in data_list:
        hourly[d["datetime"].hour].append(d)

    lines = [
        "| 時間帯 | 試合 | 勝率 | 与被ダメ比 | |",
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
        f"- **平日**: {len(weekday_data)}戦 勝率{win_rate(weekday_data):.1f}% 与被ダメ比{dmg_efficiency(weekday_data):.3f}",
        f"- **土日**: {len(weekend_data)}戦 勝率{win_rate(weekend_data):.1f}% 与被ダメ比{dmg_efficiency(weekend_data):.3f}",
        "",
        "| 曜日 | 試合 | 勝率 | 与被ダメ比 |",
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
        "| 日付 | 曜日 | 試合 | 勝率 | 与被ダメ比 | |",
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
    """固定相方（連続10戦以上）の分析"""
    fixed = detect_fixed_partners(all_data)

    if not fixed:
        return "固定相方（連続10戦以上）は検出されませんでした。"

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
        lines.append(f"| 与被ダメ比 | {my_eff:.3f} | {p_eff:.3f} |")
        lines.append(f"| 平均撃墜 | {avg([d['kills'] for d in data]):.2f} | {p_avg_kills:.2f} |")
        lines.append(f"| 平均被撃墜 | {avg([d['deaths'] for d in data]):.2f} | {p_avg_deaths:.2f} |")

        # 相方の使用機体別勝率
        partner_ms_stats = defaultdict(list)
        for d in data:
            partner_ms_stats[d["partner_ms"]].append(d)

        if len(partner_ms_stats) > 1 or any(len(v) >= 2 for v in partner_ms_stats.values()):
            lines.append("\n**相方の使用機体別:**\n")
            lines.append("| 機体 | 試合 | 勝率 | 相方与被ダメ比 |")
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
            tips.append(f"相方の与被ダメ比が{p_eff:.3f}と低めです。相方が狙われやすい展開になっている可能性があります。カットやラインを意識しましょう。")
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
        lines.append(f"- 全体: {len(data)}戦 勝率{win_rate(data):.1f}% 与被ダメ比{dmg_efficiency(data):.3f}")

        first_half = season_half[season_name].get("前半", [])
        second_half = season_half[season_name].get("後半", [])
        if first_half:
            lines.append(f"- 前半: {len(first_half)}戦 勝率{win_rate(first_half):.1f}% 与被ダメ比{dmg_efficiency(first_half):.3f}")
        if second_half:
            lines.append(f"- 後半: {len(second_half)}戦 勝率{win_rate(second_half):.1f}% 与被ダメ比{dmg_efficiency(second_half):.3f}")

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

    # コスト帯別の被撃墜アドバイス
    cost_groups = defaultdict(list)
    for d in all_data:
        cost = d.get("ms_cost", 0)
        if cost in COST_FATAL_DEATHS:
            cost_groups[cost].append(d)

    for cost in sorted(cost_groups.keys(), reverse=True):
        data = cost_groups[cost]
        fatal = COST_FATAL_DEATHS[cost]
        label = COST_LABEL[cost]
        fatal_losses = [d for d in data if d["deaths"] >= fatal and not d["win"]]
        if fatal_losses:
            rate = len(fatal_losses) / len(data) * 100
            advices.append(
                f"使用機体が{label}の時に、{fatal}落ちで敗北した試合が全体の{rate:.0f}%({len(fatal_losses)}/{len(data)}戦)"
            )

    for ms_name, data in ms_data.items():
        if len(data) < 3:
            continue
        eff = dmg_efficiency(data)
        if eff < 1.0:
            advices.append(
                f"**{ms_name}** の与被ダメ比は{eff:.3f}で1.0未満です。"
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

    # カテゴリ分けして出力
    categories = {
        "survival": {"title": "耐久管理", "items": []},
        "ms": {"title": "機体", "items": []},
        "time": {"title": "時間帯・曜日", "items": []},
        "partner": {"title": "相方", "items": []},
        "mental": {"title": "メンタル", "items": []},
        "season": {"title": "シーズン", "items": []},
    }

    for a in advices:
        if "2落ち" in a or "被撃墜" in a:
            categories["survival"]["items"].append(a)
        elif "苦手機体" in a or "与被ダメ比" in a or "立ち回り" in a:
            categories["ms"]["items"].append(a)
        elif "時間帯" in a or "平日" in a or "土日" in a:
            categories["time"]["items"].append(a)
        elif "相方" in a:
            categories["partner"]["items"].append(a)
        elif "連敗" in a:
            categories["mental"]["items"].append(a)
        elif "シーズン" in a or "前半" in a or "後半" in a:
            categories["season"]["items"].append(a)
        else:
            categories["ms"]["items"].append(a)

    lines = []
    for cat in categories.values():
        if cat["items"]:
            lines.append(f"**{cat['title']}**\n")
            for item in cat["items"]:
                lines.append(f"- {item}")
            lines.append("")
    return "\n".join(lines)


def build_share_data(all_data, ms_data):
    """SNS共有用のサマリーデータを生成"""
    items = []

    # 最多使用MS
    if ms_data:
        top_ms = max(ms_data.keys(), key=lambda x: len(ms_data[x]))
        items.append({"type": "top_ms", "ms": top_ms, "count": len(ms_data[top_ms])})

    # 得意機体・苦手機体（最多使用MSの敵機体相性）
    if ms_data:
        top_ms = max(ms_data.keys(), key=lambda x: len(ms_data[x]))
        enemy_stats = defaultdict(list)
        for d in ms_data[top_ms]:
            for ems in d["enemy_ms"]:
                enemy_stats[ems].append(d)

        best_enemy = None
        worst_enemy = None
        for ems, matches in enemy_stats.items():
            if len(matches) >= 3:
                wr = win_rate(matches)
                if best_enemy is None or wr > best_enemy[1]:
                    best_enemy = (ems, wr, len(matches))
                if worst_enemy is None or wr < worst_enemy[1]:
                    worst_enemy = (ems, wr, len(matches))

        if best_enemy and best_enemy[1] >= 60:
            items.append({"type": "strong_enemy", "enemy": best_enemy[0], "wr": round(best_enemy[1]), "count": best_enemy[2]})
        if worst_enemy and worst_enemy[1] <= 40:
            items.append({"type": "weak_enemy", "enemy": worst_enemy[0], "wr": round(worst_enemy[1]), "count": worst_enemy[2]})

    # 与被ダメ比（最多使用MS）
    if ms_data:
        top_ms = max(ms_data.keys(), key=lambda x: len(ms_data[x]))
        data = ms_data[top_ms]
        if len(data) >= 3:
            eff = dmg_efficiency(data)
            items.append({"type": "dmg_efficiency", "ms": top_ms, "value": round(eff, 3)})

    return items


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <csv_path>")
        sys.exit(1)

    csv_path = sys.argv[1]
    matches = load_csv(csv_path)

    if not matches:
        print("試合データが見つかりませんでした。")
        sys.exit(1)

    cost_map = load_ms_cost_map(csv_path)
    player_name = detect_player_name(matches)

    all_data = [get_my_data(m, cost_map) for m in matches]

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
    toc.append(f"1. {toc_link('総合アドバイス', '総合アドバイス')}")
    toc.append(f"2. {toc_link('基本データ', '基本データ')}")
    n = 3
    for i, ms_name in enumerate(ms_names_for_toc):
        ms_count = len(ms_data[ms_name])
        heading = f"機体別分析:-{ms_name}-({ms_count}戦)"
        toc.append(f"{n+i}. {toc_link(ms_name + ' (' + str(ms_count) + '戦)', heading)}")
        toc.append(f"   - {toc_link('基本データ', '基本データ（' + ms_name + '）')}")
        toc.append(f"   - {toc_link('敵機体との相性', '敵機体との相性（' + ms_name + '）')}")
        toc.append(f"   - {toc_link('相方機体との相性', '相方機体との相性（' + ms_name + '）')}")
    n += len(ms_names_for_toc)
    toc.append(f"{n}. {toc_link('コスト編成別勝率', 'コスト編成別勝率')}")
    toc.append(f"{n+1}. {toc_link('ダメージ貢献率', 'ダメージ貢献率')}")
    toc.append(f"{n+2}. {toc_link('固定相方分析', '固定相方分析（連続10戦以上）')}")
    toc.append(f"{n+3}. {toc_link('被撃墜数と勝率', '被撃墜数と勝率の関係')}")
    toc.append(f"{n+4}. {toc_link('時間帯別', '時間帯別の勝率')}")
    toc.append(f"{n+5}. {toc_link('曜日別', '曜日別の勝率（平日-vs-土日）')}")
    toc.append(f"{n+6}. {toc_link('日別推移', '日別勝率推移')}")
    toc.append(f"{n+7}. {toc_link('シーズン別', 'シーズン別分析')}")
    toc.append("\n</details>")
    report.append("\n".join(toc))

    # 総合アドバイス（冒頭に配置、デフォルト展開）
    report.append("\n\n---\n\n<details open><summary><strong>総合アドバイス</strong></summary>\n")
    report.append(md_advice(all_data, ms_data))
    report.append("\n</details>")

    # 基本データ
    report.append("\n---\n\n<details><summary><strong>基本データ</strong></summary>\n")
    report.append(md_basic_stats(all_data))
    report.append("\n### 勝ち/負け時のダメージ傾向\n")
    report.append(md_win_loss_pattern(all_data))
    report.append("\n</details>")

    # 機体別分析
    for ms_name in ms_names_for_toc:
        data = ms_data[ms_name]

        report.append(f"\n---\n\n<details><summary><strong>機体別分析: {ms_name} ({len(data)}戦)</strong></summary>\n")
        report.append(f"### 基本データ（{ms_name}）\n")
        report.append(md_basic_stats(data))
        report.append(f"\n### 勝ち/負け時のダメージ傾向（{ms_name}）\n")
        report.append(md_win_loss_pattern(data))
        report.append(f"\n### 敵機体との相性（{ms_name}）\n")
        report.append(md_enemy_matchup(data))
        report.append(f"\n### 相方機体との相性（{ms_name}）\n")
        report.append(md_partner(data))
        report.append("\n</details>")

    # コスト編成別勝率
    report.append("\n---\n\n<details><summary><strong>コスト編成別勝率</strong></summary>\n")
    report.append(md_cost_pair(all_data))
    report.append("\n</details>")

    # ダメージ貢献率
    report.append("\n<details><summary><strong>ダメージ貢献率</strong></summary>\n")
    report.append(md_dmg_contribution(all_data))
    report.append("\n</details>")

    # 固定相方分析
    report.append("\n---\n\n<details><summary><strong>固定相方分析（連続10戦以上）</strong></summary>\n")
    report.append(md_fixed_partners(all_data))
    report.append("\n</details>")

    # 被撃墜数と勝率
    report.append("\n---\n\n<details><summary><strong>被撃墜数と勝率の関係</strong></summary>\n")
    report.append(md_deaths_impact(all_data))
    report.append("\n</details>")

    # 時間帯別
    report.append("\n<details><summary><strong>時間帯別の勝率</strong></summary>\n")
    report.append(md_time_of_day(all_data))
    report.append("\n</details>")

    # 曜日別
    report.append("\n<details><summary><strong>曜日別の勝率（平日 vs 土日）</strong></summary>\n")
    report.append(md_day_of_week(all_data))
    report.append("\n</details>")

    # 日別推移
    report.append("\n<details><summary><strong>日別勝率推移</strong></summary>\n")
    report.append(md_daily_trend(all_data))
    report.append("\n</details>")

    # シーズン分析
    report.append("\n<details><summary><strong>シーズン別分析</strong></summary>\n")
    report.append(md_season(all_data))
    report.append("\n</details>")

    # SNS共有用データ
    share_data = build_share_data(all_data, ms_data)
    if share_data:
        report.append(f"\n<!-- SHARE_DATA:{json.dumps(share_data, ensure_ascii=False)} -->")

    # ファイル出力
    output_path = os.path.join(os.path.dirname(csv_path), "report.md")
    content = "\n".join(report) + "\n"

    with open(output_path, "w", encoding="utf-8") as f:
        f.write(content)

    print(f"分析レポートを出力しました: {output_path}")


if __name__ == "__main__":
    main()
