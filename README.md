# ⚽ Surebets Real-Time Engine (Go)

## 📌 Overview

This project is a **real-time surebet detection engine** for football matches, built in **Go**.

It calculates arbitrage opportunities (**surebets**) across different bookmakers using live odds data. The system processes multiple matches concurrently and detects profitable combinations when market conditions allow it.

> **NOTE:**
> This is a simplified single-file version of the engine, focused on demonstrating concurrency and surebet detection logic.  
> Not production-ready architecture.

---

## ⚙️ How it works

- Odds are received as formatted strings via **Redis Pub/Sub**
- Each incoming message represents a betting odd from a bookmaker
- Data is grouped by:
  - Match (home vs away)
  - Market type
- Each match maintains independent market structures

### Processing logic

- The system continuously monitors incoming data
- If no new odds are received within a short time window:
  - Surebet calculation is triggered
- If a surebet is found:
  - It is sent to a **Telegram channel**

---

## 🧠 Supported markets

The engine currently processes the following football betting markets:

- **BTG** → Both teams to score  
- **TPI** → Total goals odd/even  
- **TUO** → Total goals Over/Under  
- **FUO** → Additional Over/Under lines  

---

## 📡 Input format

Odds are received as strings, for example:

bwin:Girona:Las Palmas:tuo:2.5:Over:1.95
betfair:Girona:Las Palmas:tuo:2.5:Under:2.10

---

## 🌍 Language considerations

- The current implementation is adapted to bookmakers using **Spanish labels and formats**
- The engine logic itself is **language-agnostic** and can be easily adapted to any language
- Code comments are written in **Spanish**

---

## 📤 Output

Detected surebets are sent to Telegram including:

- Match  
- Market  
- Odds combination  
- Profit percentage  

---

## 🚀 Features

- Real-time processing using **goroutines**
- Concurrent-safe data structures (**mutexes**)
- Market-level independent calculations
- Inactivity-based triggering (avoids redundant computations)
- Multi-match support
- Event-driven architecture using **Redis Pub/Sub**

---

## 🛠️ Tech stack

- **Go (Golang)**
- **Redis (Pub/Sub)**
- **Telegram Bot API**

---

## ⚙️ Environment variables

The application requires the following environment variables:

- TELEGRAM_BOT_TOKEN
- TELEGRAM_CHAT_ID
- REDIS_ADDR
- REDIS_PASS

---

## 🎯 Purpose

This project was built as a technical demonstration of:

- Real-time systems design  
- Efficient data aggregation  
- Concurrent processing in Go  
- Detection of arbitrage opportunities in betting markets  

Additionally, while the current implementation focuses on football markets,  
the architecture can be adapted to **any type of sporting event**.

Only the market-specific calculation logic would need to be modified  
(e.g. the methods responsible for evaluating betting outcomes).

---

## ⚠️ Disclaimer

This project is a functional example, intended to demonstrate:

- Concurrency patterns in Go  
- Real-time data processing  
- Arbitrage logic (surebets)  

It is **not production-ready** and does not include:

- Full error handling  
- Robust parsing/validation  
- Deployment configuration  
- Scrapers or external data collectors  

---

## 📬 Notes

- Odds ingestion (scrapers) is external to this project  
- The engine expects properly formatted data via Redis  
- Telegram integration is used for real-time alerting  

---
