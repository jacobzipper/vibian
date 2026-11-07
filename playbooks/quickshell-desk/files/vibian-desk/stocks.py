#!/usr/bin/python3 -I
"""Quotes for the desk's stocks widget, from Yahoo Finance's chart API (no
key). Tickers come from the arguments, or tickers.json next to this file.
Prints one JSON object: {"time": unix, "quotes": [{symbol, name, price, prev,
change, pct, high, low, currency, points}, or {symbol, error}]}."""
import json, os, sys, time, urllib.parse, urllib.request
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
UA = {"User-Agent": "Mozilla/5.0"}  # Yahoo answers 429 to Python's own


def tickers():
    try:
        with open(os.path.join(HERE, "tickers.json")) as f:
            return [str(t) for t in json.load(f).get("tickers", []) if t][:16]
    except (OSError, ValueError, AttributeError):
        return []


def quote(sym):
    url = ("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=5m"
           % urllib.parse.quote(sym, safe=""))
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=UA), timeout=8) as r:
            res = json.load(r)["chart"]["result"][0]
    except Exception as e:  # network, HTTP, unknown ticker
        return {"symbol": sym, "error": str(e)[:80]}
    m = res.get("meta", {})
    closes = [c for c in ((res.get("indicators", {}).get("quote") or [{}])[0].get("close") or []) if c is not None]
    price = m.get("regularMarketPrice") or (closes[-1] if closes else None)
    prev = m.get("chartPreviousClose") or m.get("previousClose")
    if price is None:
        return {"symbol": sym, "error": "no data"}
    change = price - prev if prev else 0.0
    step = max(1, len(closes) // 80)  # at most ~80 points for the sparkline
    return {
        "symbol": sym,
        "name": m.get("shortName") or m.get("longName") or sym,
        "price": price,
        "prev": prev,
        "change": change,
        "pct": change / prev * 100 if prev else 0.0,
        "high": m.get("regularMarketDayHigh"),
        "low": m.get("regularMarketDayLow"),
        "currency": m.get("currency", ""),
        "points": closes[::step],
    }


def main():
    syms = sys.argv[1:] or tickers()
    with ThreadPoolExecutor(max_workers=8) as ex:
        quotes = list(ex.map(quote, syms))
    print(json.dumps({"time": int(time.time()), "quotes": quotes}))


if __name__ == "__main__":
    main()
