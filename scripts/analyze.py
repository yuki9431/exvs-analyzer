#!/usr/bin/env python3
"""
EXVS2IB 戦績分析スクリプト
CSVファイルを読み込み、JSON形式の構造化分析レポートを出力する。
プレイヤー名はCSVのプレイヤーNo.1から自動取得する。
"""

import argparse
import csv
import json
import os
import sys
from collections import defaultdict
from datetime import datetime, timedelta


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


def load_tag_partners(path):
    """タッグ相方JSONファイルを読み込み、{player_name: team_name} の辞書を返す。"""
    if not path or not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return {entry["player_name"]: entry["team_name"] for entry in data}


def detect_fixed_partners(all_data, tag_partners=None):
    """タッグ相方名リストに基づいて固定相方の試合を返す。
    tag_partners が指定されている場合、相方名が一致する試合を固定扱いにする。
    tag_partners が空の場合、固定相方なしとして空を返す。
    """
    if not tag_partners:
        return {}

    partner_names = set(tag_partners.keys())
    fixed_matches = defaultdict(list)

    for d in all_data:
        if d["partner_name"] in partner_names:
            fixed_matches[d["partner_name"]].append(d)

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


# ========== JSON構造化データ出力関数 ==========


def data_basic_stats(data_list):
    n = len(data_list)
    w, l = wins_losses(data_list)
    wr = w / n * 100 if n > 0 else 0
    total_kills = sum(d["kills"] for d in data_list)
    total_deaths = sum(d["deaths"] for d in data_list)
    kd = total_kills / total_deaths if total_deaths > 0 else 0
    eff = dmg_efficiency(data_list)

    tips = []
    if eff < 1.0:
        tips.append(f"与被ダメ比が{eff:.3f}で1.0未満です。被ダメが与ダメを上回っており、被弾を減らす立ち回りが必要です。")
    elif eff >= 1.2:
        tips.append(f"与被ダメ比{eff:.3f}は優秀です。この調子を維持しましょう。")
    if kd < 1.0:
        tips.append(f"K/D比が{kd:.2f}で1.0未満です。撃墜数を増やすか、被撃墜を減らすことを意識しましょう。")

    return {
        "matches": n,
        "wins": w,
        "losses": l,
        "win_rate": round(wr, 1),
        "avg_dmg_given": round(avg([d["dmg_given"] for d in data_list])),
        "avg_dmg_taken": round(avg([d["dmg_taken"] for d in data_list])),
        "dmg_efficiency": round(eff, 3),
        "avg_kills": round(avg([d["kills"] for d in data_list]), 2),
        "avg_deaths": round(avg([d["deaths"] for d in data_list]), 2),
        "kd_ratio": round(kd, 2),
        "avg_ex_dmg": round(avg([d["ex_dmg"] for d in data_list])),
        "tips": tips,
    }


def data_win_loss_pattern(data_list):
    wins = [d for d in data_list if d["win"]]
    losses = [d for d in data_list if not d["win"]]

    metrics = []
    for label, key in [("与ダメージ", "dmg_given"), ("被ダメージ", "dmg_taken"),
                        ("撃墜", "kills"), ("被撃墜", "deaths")]:
        w_avg = avg([d[key] for d in wins]) if wins else 0
        l_avg = avg([d[key] for d in losses]) if losses else 0
        metrics.append({
            "label": label,
            "win_avg": round(w_avg, 1),
            "loss_avg": round(l_avg, 1),
            "diff": round(w_avg - l_avg, 1),
        })

    w_eff = dmg_efficiency(wins) if wins else 0
    l_eff = dmg_efficiency(losses) if losses else 0
    metrics.append({
        "label": "与被ダメ比",
        "win_avg": round(w_eff, 3),
        "loss_avg": round(l_eff, 3),
        "diff": round(w_eff - l_eff, 3),
    })

    tips = []
    w_deaths = avg([d["deaths"] for d in wins]) if wins else 0
    l_deaths = avg([d["deaths"] for d in losses]) if losses else 0
    l_taken = avg([d["dmg_taken"] for d in losses]) if losses else 0
    if l_deaths >= 1.5:
        tips.append(f"負け試合の平均被撃墜が{l_deaths:.1f}と高いです。耐久管理を意識しましょう。")
    if l_taken >= 1100:
        tips.append(f"負け試合の被ダメージが平均{l_taken:.0f}と高いです。無駄な被弾を減らすことが改善の鍵です。")

    # コスト帯別
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_LABEL:
            cost_groups[cost].append(d)

    cost_patterns = []
    for cost in sorted(cost_groups.keys(), reverse=True):
        data = cost_groups[cost]
        if len(data) < 3:
            continue
        c_wins = [d for d in data if d["win"]]
        c_losses = [d for d in data if not d["win"]]
        if not c_wins or not c_losses:
            continue

        fatal = COST_FATAL_DEATHS[cost]
        fatal_losses = [d for d in c_losses if d["deaths"] >= fatal]

        cost_metrics = []
        for m_label, key in [("与ダメージ", "dmg_given"), ("被ダメージ", "dmg_taken"),
                              ("撃墜", "kills"), ("被撃墜", "deaths")]:
            w_v = avg([d[key] for d in c_wins])
            l_v = avg([d[key] for d in c_losses])
            cost_metrics.append({
                "label": m_label,
                "win_avg": round(w_v, 1),
                "loss_avg": round(l_v, 1),
                "diff": round(w_v - l_v, 1),
            })
        c_w_eff = dmg_efficiency(c_wins)
        c_l_eff = dmg_efficiency(c_losses)
        cost_metrics.append({
            "label": "与被ダメ比",
            "win_avg": round(c_w_eff, 3),
            "loss_avg": round(c_l_eff, 3),
            "diff": round(c_w_eff - c_l_eff, 3),
        })

        cost_patterns.append({
            "cost": cost,
            "cost_label": COST_LABEL[cost],
            "matches": len(data),
            "win_rate": round(win_rate(data), 1),
            "metrics": cost_metrics,
            "fatal_deaths": fatal,
            "fatal_loss_count": len(fatal_losses),
            "fatal_loss_total": len(c_losses),
            "fatal_loss_rate": round(len(fatal_losses) / len(c_losses) * 100) if c_losses else 0,
        })

    return {
        "metrics": metrics,
        "tips": tips,
        "cost_patterns": cost_patterns,
    }


def data_enemy_matchup(data_list, min_matches=3):
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
            results.append({
                "ms": ms,
                "matches": len(matches),
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
                "avg_dmg_given": round(avg_given),
                "avg_dmg_taken": round(avg_taken),
            })

    strong = sorted([r for r in results if r["win_rate"] >= 60], key=lambda x: -x["matches"])
    weak = sorted([r for r in results if r["win_rate"] <= 40], key=lambda x: -x["matches"])
    even = sorted([r for r in results if 40 < r["win_rate"] < 60], key=lambda x: -x["matches"])

    tips = []
    if weak:
        high_dmg_taken = [r for r in weak if r["avg_dmg_taken"] >= 1200]
        if high_dmg_taken:
            names = "、".join(r["ms"] for r in high_dmg_taken[:3])
            tips.append(f"{names} 戦では被ダメが特に多いです。距離管理を見直しましょう。")
        low_dmg_given = [r for r in weak if r["avg_dmg_given"] <= 900]
        if low_dmg_given:
            names = "、".join(r["ms"] for r in low_dmg_given[:3])
            tips.append(f"{names} 戦では与ダメが低いです。攻撃の手数や当て方を工夫しましょう。")

    return {
        "strong": strong,
        "weak": weak,
        "even": even,
        "tips": tips,
    }


def data_partner(data_list, min_matches=3):
    partner_stats = defaultdict(list)
    for d in data_list:
        partner_stats[d["partner_ms"]].append(d)

    results = []
    for ms, matches in partner_stats.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            results.append({
                "ms": ms,
                "matches": len(matches),
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
            })

    results.sort(key=lambda x: -x["matches"])
    return results


def data_cost_pair(data_list, min_matches=3):
    pairs = defaultdict(list)
    for d in data_list:
        ms_name = d.get("ms", "(不明)")
        partner_cost = d.get("partner_cost", 0)
        if ms_name and partner_cost:
            key = f"{ms_name} + {partner_cost}"
            pairs[key].append(d)

    results = []
    for pair, matches in pairs.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            results.append({
                "pair": pair,
                "matches": len(matches),
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
            })

    results.sort(key=lambda x: -x["matches"])
    return results


def data_ms_pair(data_list, min_matches=3, top_n=10):
    pairs = defaultdict(list)
    for d in data_list:
        key = f"{d['ms']} + {d['partner_ms']}"
        pairs[key].append(d)

    results = []
    for pair, matches in pairs.items():
        if len(matches) >= min_matches:
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            w, l = wins_losses(matches)
            results.append({
                "pair": pair,
                "matches": len(matches),
                "wins": w,
                "losses": l,
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
            })

    by_wr = sorted(results, key=lambda x: (-x["win_rate"], -x["matches"]))[:top_n]
    by_count = sorted(results, key=lambda x: (-x["matches"], -x["win_rate"]))[:top_n]

    return {
        "by_win_rate": by_wr,
        "by_matches": by_count,
    }


def data_dmg_contribution(data_list, min_matches=3):
    contribs = []
    for d in data_list:
        team_total = d["dmg_given"] + d["partner_dmg_given"]
        if team_total > 0:
            contribs.append(d["dmg_given"] / team_total * 100)
    avg_contrib = sum(contribs) / len(contribs) if contribs else 0

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

    # コスト帯別
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_LABEL:
            cost_groups[cost].append(d)

    cost_data = []
    if len(cost_groups) > 1 or any(len(v) >= min_matches for v in cost_groups.values()):
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
            cost_data.append({
                "cost": cost,
                "cost_label": COST_LABEL[cost],
                "matches": len(data),
                "avg_contribution": round(sum(c_all) / len(c_all), 1) if c_all else 0,
                "avg_win_contribution": round(sum(c_win) / len(c_win), 1) if c_win else 0,
                "avg_lose_contribution": round(sum(c_lose) / len(c_lose), 1) if c_lose else 0,
            })

    return {
        "avg_contribution": round(avg_contrib, 1),
        "avg_win_contribution": round(avg_win, 1),
        "avg_lose_contribution": round(avg_lose, 1),
        "by_cost": cost_data,
    }


def data_fixed_partners(all_data, tag_partners=None):
    fixed = detect_fixed_partners(all_data, tag_partners)

    if not fixed:
        if not tag_partners:
            return {"notice": "タッグ情報が見つかりませんでした。フレンドを登録してタッグを組むと、固定相方の詳細分析が利用できます。", "partners": []}
        return {"partners": []}

    results = []
    for partner_name in sorted(fixed.keys(), key=lambda x: -len(fixed[x])):
        data = fixed[partner_name]
        n = len(data)
        w, l = wins_losses(data)
        wr = w / n * 100

        my_eff = dmg_efficiency(data)
        my_avg_given = avg([d["dmg_given"] for d in data])
        my_avg_taken = avg([d["dmg_taken"] for d in data])

        p_avg_given = avg([d["partner_dmg_given"] for d in data])
        p_avg_taken = avg([d["partner_dmg_taken"] for d in data])
        p_total_given = sum(d["partner_dmg_given"] for d in data)
        p_total_taken = sum(d["partner_dmg_taken"] for d in data)
        p_eff = p_total_given / p_total_taken if p_total_taken > 0 else 0
        p_avg_kills = avg([d["partner_kills"] for d in data])
        p_avg_deaths = avg([d["partner_deaths"] for d in data])

        # 相方の使用機体別
        partner_ms_stats = defaultdict(list)
        for d in data:
            partner_ms_stats[d["partner_ms"]].append(d)

        ms_breakdown = []
        if len(partner_ms_stats) > 1 or any(len(v) >= 2 for v in partner_ms_stats.values()):
            for ms in sorted(partner_ms_stats.keys(), key=lambda x: -len(partner_ms_stats[x])):
                ms_data_list = partner_ms_stats[ms]
                ms_wr = win_rate(ms_data_list)
                ms_p_given = sum(d["partner_dmg_given"] for d in ms_data_list)
                ms_p_taken = sum(d["partner_dmg_taken"] for d in ms_data_list)
                ms_p_eff = ms_p_given / ms_p_taken if ms_p_taken > 0 else 0
                ms_breakdown.append({
                    "ms": ms,
                    "matches": len(ms_data_list),
                    "win_rate": round(ms_wr, 1),
                    "partner_dmg_efficiency": round(ms_p_eff, 3),
                })

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

        entry = {
            "partner_name": partner_name,
            "matches": n,
            "wins": w,
            "losses": l,
            "win_rate": round(wr, 1),
            "my_stats": {
                "avg_dmg_given": round(my_avg_given),
                "avg_dmg_taken": round(my_avg_taken),
                "dmg_efficiency": round(my_eff, 3),
                "avg_kills": round(avg([d["kills"] for d in data]), 2),
                "avg_deaths": round(avg([d["deaths"] for d in data]), 2),
            },
            "partner_stats": {
                "avg_dmg_given": round(p_avg_given),
                "avg_dmg_taken": round(p_avg_taken),
                "dmg_efficiency": round(p_eff, 3),
                "avg_kills": round(p_avg_kills, 2),
                "avg_deaths": round(p_avg_deaths, 2),
            },
            "partner_ms_breakdown": ms_breakdown,
            "tips": tips,
        }
        if tag_partners and partner_name in tag_partners:
            entry["team_name"] = tag_partners[partner_name]
        results.append(entry)

    return {"partners": results}


def data_deaths_impact(data_list):
    cost_groups = defaultdict(list)
    for d in data_list:
        cost = d.get("ms_cost", 0)
        if cost in COST_FATAL_DEATHS:
            cost_groups[cost].append(d)

    results = []
    for cost in sorted(cost_groups.keys(), reverse=True):
        data = cost_groups[cost]
        fatal = COST_FATAL_DEATHS[cost]
        max_bucket = fatal + 1

        by_deaths = defaultdict(list)
        for d in data:
            deaths = d["deaths"]
            if deaths >= max_bucket:
                key = f"{max_bucket}+"
            else:
                key = str(deaths)
            by_deaths[key].append(d)

        buckets = []
        for i in range(max_bucket):
            key = str(i)
            if key in by_deaths:
                matches = by_deaths[key]
                wr = win_rate(matches)
                eff = dmg_efficiency(matches)
                if i >= fatal:
                    status = "fatal"
                elif i == fatal - 1:
                    status = "danger"
                else:
                    status = "safe"
                buckets.append({
                    "deaths": i,
                    "label": f"{i}回",
                    "matches": len(matches),
                    "win_rate": round(wr, 1),
                    "dmg_efficiency": round(eff, 3),
                    "status": status,
                })
        over_key = f"{max_bucket}+"
        if over_key in by_deaths:
            matches = by_deaths[over_key]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            buckets.append({
                "deaths": max_bucket,
                "label": f"{max_bucket}回以上",
                "matches": len(matches),
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
                "status": "fatal",
            })

        fatal_count = sum(len(by_deaths.get(str(i), [])) for i in range(fatal, max_bucket))
        fatal_count += len(by_deaths.get(over_key, []))
        safe_data = []
        for i in range(fatal):
            safe_data.extend(by_deaths.get(str(i), []))
        safe_wr = win_rate(safe_data) if safe_data else 0

        tips = []
        if fatal_count > 0 and len(data) > 0:
            tips.append(f"負け確定({fatal}落ち以上)に達した試合は{fatal_count}/{len(data)}戦({fatal_count/len(data)*100:.0f}%)。")
        if safe_data:
            tips.append(f"{fatal-1}落ち以内に抑えた場合の勝率は{safe_wr:.1f}%。")

        results.append({
            "cost": cost,
            "cost_label": COST_LABEL[cost],
            "matches": len(data),
            "fatal_deaths": fatal,
            "fatal_cost": cost * fatal,
            "buckets": buckets,
            "tips": tips,
        })

    return results


def data_time_of_day(data_list):
    hourly = defaultdict(list)
    for d in data_list:
        hourly[d["datetime"].hour].append(d)

    results = []
    for hour in sorted(hourly.keys()):
        matches = hourly[hour]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        mark = "good" if wr >= 70 else "bad" if wr <= 40 else ""
        results.append({
            "hour": hour,
            "matches": len(matches),
            "win_rate": round(wr, 1),
            "dmg_efficiency": round(eff, 3),
            "mark": mark,
        })

    tips = []
    good = [h for h, m in hourly.items() if len(m) >= 5 and win_rate(m) >= 70]
    bad = [h for h, m in hourly.items() if len(m) >= 5 and win_rate(m) <= 40]
    if good:
        tips.append(f"{'、'.join(f'{h}時台' for h in sorted(good))}が好調です。")
    if bad:
        tips.append(f"{'、'.join(f'{h}時台' for h in sorted(bad))}は不調です。強い相手が多い時間帯か、疲労の影響かもしれません。")

    return {"hours": results, "tips": tips}


def data_day_of_week(data_list):
    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    daily = defaultdict(list)
    for d in data_list:
        daily[d["datetime"].weekday()].append(d)

    weekday_data = [d for d in data_list if d["datetime"].weekday() < 5]
    weekend_data = [d for d in data_list if d["datetime"].weekday() >= 5]

    days = []
    for dow in range(7):
        if dow in daily:
            matches = daily[dow]
            wr = win_rate(matches)
            eff = dmg_efficiency(matches)
            days.append({
                "dow": dow,
                "name": DOW_NAMES[dow],
                "matches": len(matches),
                "win_rate": round(wr, 1),
                "dmg_efficiency": round(eff, 3),
            })

    wd_wr = win_rate(weekday_data) if weekday_data else 0
    we_wr = win_rate(weekend_data) if weekend_data else 0
    diff = abs(wd_wr - we_wr)
    tips = []
    if diff >= 10:
        better = "平日" if wd_wr > we_wr else "土日"
        worse = "土日" if wd_wr > we_wr else "平日"
        tips.append(f"{better}の方が{worse}より勝率が{diff:.0f}ポイント高いです。{worse}は対戦相手の質が変わる可能性があります。")

    return {
        "weekday": {
            "matches": len(weekday_data),
            "win_rate": round(wd_wr, 1),
            "dmg_efficiency": round(dmg_efficiency(weekday_data), 3),
        },
        "weekend": {
            "matches": len(weekend_data),
            "win_rate": round(we_wr, 1),
            "dmg_efficiency": round(dmg_efficiency(weekend_data), 3),
        },
        "days": days,
        "tips": tips,
    }


def data_daily_trend(data_list):
    daily = defaultdict(list)
    for d in data_list:
        date_str = d["datetime"].strftime("%m/%d")
        daily[date_str].append(d)

    DOW_NAMES = ["月", "火", "水", "木", "金", "土", "日"]
    results = []
    for date_str in sorted(daily.keys()):
        matches = daily[date_str]
        wr = win_rate(matches)
        eff = dmg_efficiency(matches)
        dow = matches[0]["datetime"].weekday()
        mark = "good" if wr >= 70 else "bad" if wr <= 45 else ""
        results.append({
            "date": date_str,
            "dow_name": DOW_NAMES[dow],
            "matches": len(matches),
            "win_rate": round(wr, 1),
            "dmg_efficiency": round(eff, 3),
            "mark": mark,
        })

    # 連敗ストリーク
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

    return {
        "days": results,
        "max_lose_streak": max_lose_streak,
        "tips": tips,
    }


def data_season(data_list):
    season_data = defaultdict(list)
    season_half = defaultdict(lambda: defaultdict(list))

    for d in data_list:
        s = get_season(d["datetime"])
        h = get_season_half(d["datetime"])
        season_data[s].append(d)
        season_half[s][h].append(d)

    results = []
    for season_name in sorted(season_data.keys()):
        data = season_data[season_name]
        first_half = season_half[season_name].get("前半", [])
        second_half = season_half[season_name].get("後半", [])

        tips = []
        if first_half and second_half:
            f_wr = win_rate(first_half)
            s_wr = win_rate(second_half)
            diff = s_wr - f_wr
            if abs(diff) >= 5:
                if diff > 0:
                    tips.append(f"後半の方が勝率が{diff:.0f}ポイント高く、シーズンが進むにつれて安定しています。")
                else:
                    tips.append(f"前半の方が勝率が{-diff:.0f}ポイント高いです。後半は対戦相手のレベルが上がっている可能性があります。")

        entry = {
            "name": season_name,
            "matches": len(data),
            "win_rate": round(win_rate(data), 1),
            "dmg_efficiency": round(dmg_efficiency(data), 3),
            "tips": tips,
        }
        if first_half:
            entry["first_half"] = {
                "matches": len(first_half),
                "win_rate": round(win_rate(first_half), 1),
                "dmg_efficiency": round(dmg_efficiency(first_half), 3),
            }
        if second_half:
            entry["second_half"] = {
                "matches": len(second_half),
                "win_rate": round(win_rate(second_half), 1),
                "dmg_efficiency": round(dmg_efficiency(second_half), 3),
            }
        results.append(entry)

    return results


def data_advice(all_data, ms_data, tag_partners=None):
    advices = []

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
            advices.append({
                "category": "survival",
                "text": f"使用機体が{label}の時に、{fatal}落ちで敗北した試合が全体の{rate:.0f}%({len(fatal_losses)}/{len(data)}戦)",
            })

    for ms_name, data in ms_data.items():
        if len(data) < 3:
            continue
        eff = dmg_efficiency(data)
        if eff < 1.0:
            advices.append({
                "category": "ms",
                "text": f"{ms_name} の与被ダメ比は{eff:.3f}で1.0未満です。被ダメージが与ダメージを上回っており、立ち回りの改善が必要です。",
            })

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
        advices.append({
            "category": "time",
            "text": f"勝率が低い時間帯: {hours_str}。この時間帯を避けるか、意識的にプレイしましょう。",
        })
    if good_hours:
        hours_str = "、".join(f"{h}時台({wr:.0f}%)" for h, wr in good_hours)
        advices.append({
            "category": "time",
            "text": f"勝率が高い時間帯: {hours_str}。この時間帯を活用しましょう。",
        })

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
            advices.append({
                "category": "ms",
                "text": f"{ms_name} の苦手機体: {', '.join(weak_enemies)}。対策を練るか、別の機体での対応を検討しましょう。",
            })

    weekday_data = [d for d in all_data if d["datetime"].weekday() < 5]
    weekend_data = [d for d in all_data if d["datetime"].weekday() >= 5]
    if weekday_data and weekend_data:
        wd_wr = win_rate(weekday_data)
        we_wr = win_rate(weekend_data)
        diff = abs(wd_wr - we_wr)
        if diff >= 10:
            better = "平日" if wd_wr > we_wr else "土日"
            worse = "土日" if wd_wr > we_wr else "平日"
            advices.append({
                "category": "time",
                "text": f"{better}の勝率({max(wd_wr, we_wr):.0f}%)が{worse}({min(wd_wr, we_wr):.0f}%)より{diff:.0f}ポイント高いです。",
            })

    fixed = detect_fixed_partners(all_data, tag_partners)
    if len(fixed) >= 2:
        partner_wrs = [(name, win_rate(data), len(data)) for name, data in fixed.items() if len(data) >= 5]
        if len(partner_wrs) >= 2:
            partner_wrs.sort(key=lambda x: -x[1])
            best = partner_wrs[0]
            worst = partner_wrs[-1]
            if best[1] - worst[1] >= 15:
                advices.append({
                    "category": "partner",
                    "text": f"固定相方の勝率差が大きいです。{best[0]}({best[1]:.0f}%)と{worst[0]}({worst[1]:.0f}%)で{best[1]-worst[1]:.0f}ポイント差。相方ごとに戦い方を変えるか、相性の良い相方との試合を増やしましょう。",
                })

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
        advices.append({
            "category": "mental",
            "text": f"最大{max_lose_streak}連敗の記録があります。3連敗したら休憩を挟みましょう。メンタル管理も勝率に直結します。",
        })

    season_data_map = defaultdict(list)
    season_half_map = defaultdict(lambda: defaultdict(list))
    for d in all_data:
        s = get_season(d["datetime"])
        h = get_season_half(d["datetime"])
        season_data_map[s].append(d)
        season_half_map[s][h].append(d)

    for season_name in season_data_map:
        first = season_half_map[season_name].get("前半", [])
        second = season_half_map[season_name].get("後半", [])
        if first and second:
            f_wr = win_rate(first)
            s_wr = win_rate(second)
            diff = s_wr - f_wr
            if abs(diff) >= 10:
                if diff > 0:
                    advices.append({
                        "category": "season",
                        "text": f"{season_name}: 後半の勝率が前半より{diff:.0f}ポイント高く、シーズン後半に安定する傾向があります。",
                    })
                else:
                    advices.append({
                        "category": "season",
                        "text": f"{season_name}: 前半の勝率が後半より{-diff:.0f}ポイント高いです。後半は対戦環境が厳しくなっている可能性があります。",
                    })

    # カテゴリ順序
    category_order = ["survival", "ms", "time", "partner", "mental", "season"]
    category_titles = {
        "survival": "耐久管理",
        "ms": "機体",
        "time": "時間帯・曜日",
        "partner": "相方",
        "mental": "メンタル",
        "season": "シーズン",
    }

    # カテゴリ未分類のものをmsに割り当て
    for a in advices:
        if a["category"] not in category_titles:
            a["category"] = "ms"

    categories = []
    for cat in category_order:
        items = [a["text"] for a in advices if a["category"] == cat]
        if items:
            categories.append({
                "key": cat,
                "title": category_titles[cat],
                "items": items,
            })

    return {"categories": categories}


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


def build_period_report(all_data, ms_data, tag_partners=None):
    """1期間分の分析レポートを生成する"""
    ms_names = [ms for ms in sorted(ms_data.keys(), key=lambda x: -len(ms_data[x])) if len(ms_data[ms]) >= 3]

    ms_stats = {}
    for ms_name in ms_names:
        data = ms_data[ms_name]
        ms_stats[ms_name] = {
            "matches": len(data),
            "basic_stats": data_basic_stats(data),
            "win_loss_pattern": data_win_loss_pattern(data),
            "enemy_matchup": data_enemy_matchup(data),
            "partner": data_partner(data),
            "ms_pair": data_ms_pair(data),
            "cost_pair": data_cost_pair(data),
            "dmg_contribution": data_dmg_contribution(data),
        }

    return {
        "summary": data_advice(all_data, ms_data, tag_partners),
        "basic_stats": data_basic_stats(all_data),
        "win_loss_pattern": data_win_loss_pattern(all_data),
        "ms_stats": ms_stats,
        "fixed_partners": data_fixed_partners(all_data, tag_partners),
        "deaths_impact": data_deaths_impact(all_data),
        "time_of_day": data_time_of_day(all_data),
        "day_of_week": data_day_of_week(all_data),
        "daily_trend": data_daily_trend(all_data),
        "season": data_season(all_data),
    }


def filter_by_days(all_data, days):
    """直近N日分のデータをフィルタする"""
    if not all_data:
        return []
    latest = max(d["datetime"] for d in all_data)
    cutoff = latest - timedelta(days=days)
    return [d for d in all_data if d["datetime"] >= cutoff]


def filter_by_datetime_range(all_data, start, end):
    """開始日時〜終了日時でデータをフィルタする"""
    if not all_data:
        return []
    return [d for d in all_data if start <= d["datetime"] <= end]


def build_ms_data(data_list):
    """データリストからMS別データを構築する"""
    ms_data = defaultdict(list)
    for d in data_list:
        ms_data[d["ms"]].append(d)
    return ms_data


def build_json_report(player_name, all_data, ms_data, tag_partners=None):
    """期間別の構造化JSONレポートを生成する"""
    periods = {}

    # 全期間
    periods["all"] = build_period_report(all_data, ms_data, tag_partners)
    periods["all"]["label"] = "全データ"

    # プリセット期間
    preset_periods = [
        ("90d", 90, "90日間"),
        ("60d", 60, "60日間"),
        ("30d", 30, "30日間"),
        ("14d", 14, "14日間"),
        ("7d", 7, "7日間"),
        ("3d", 3, "3日間"),
        ("1d", 1, "1日間"),
    ]
    for key, days, label in preset_periods:
        filtered = filter_by_days(all_data, days)
        if filtered:
            ms_filtered = build_ms_data(filtered)
            periods[key] = build_period_report(filtered, ms_filtered, tag_partners)
            periods[key]["label"] = label

    return {
        "player_name": player_name,
        "generated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
        "share_data": build_share_data(all_data, ms_data),
        "periods": periods,
    }


def build_custom_period_report(player_name, all_data, ms_data, start, end):
    """カスタム日時範囲の構造化JSONレポートを生成する"""
    filtered = filter_by_datetime_range(all_data, start, end)
    if not filtered:
        return None

    ms_filtered = build_ms_data(filtered)
    period_report = build_period_report(filtered, ms_filtered)
    start_str = start.strftime("%Y-%m-%d %H:%M")
    end_str = end.strftime("%Y-%m-%d %H:%M")
    period_report["label"] = f"{start_str} 〜 {end_str}"

    return {
        "player_name": player_name,
        "generated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
        "share_data": build_share_data(filtered, ms_filtered),
        "periods": {"custom": period_report},
    }


def main():
    parser = argparse.ArgumentParser(description="EXVS2IB 戦績分析")
    parser.add_argument("csv_path", help="CSVファイルパス")
    parser.add_argument("--start", help="開始日時 (YYYY-MM-DD HH:MM)")
    parser.add_argument("--end", help="終了日時 (YYYY-MM-DD HH:MM)")
    parser.add_argument("--tag-partners", help="タッグ相方JSONファイルパス", default=None)
    args = parser.parse_args()

    matches = load_csv(args.csv_path)

    if not matches:
        print("試合データが見つかりませんでした。")
        sys.exit(1)

    cost_map = load_ms_cost_map(args.csv_path)
    player_name = detect_player_name(matches)
    tag_partners = load_tag_partners(args.tag_partners)

    all_data = [get_my_data(m, cost_map) for m in matches]

    ms_data = defaultdict(list)
    for d in all_data:
        ms_data[d["ms"]].append(d)

    if args.start and args.end:
        start = datetime.strptime(args.start, "%Y-%m-%d %H:%M")
        end = datetime.strptime(args.end, "%Y-%m-%d %H:%M")
        report_data = build_custom_period_report(player_name, all_data, ms_data, start, end)
        if report_data is None:
            print("指定期間のデータが見つかりませんでした。")
            sys.exit(1)
    else:
        report_data = build_json_report(player_name, all_data, ms_data, tag_partners)

    output_path = os.path.join(os.path.dirname(args.csv_path), "report.json")
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(report_data, f, ensure_ascii=False, indent=2)
    print(f"分析レポート(JSON)を出力しました: {output_path}")


if __name__ == "__main__":
    main()
