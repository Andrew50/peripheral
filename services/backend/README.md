/cmd
- server          ← start http and ws server
- worker          ← cron / queue consumer (runs “jobs”
/internal          ← all non-exported business logic
- jobs            ← sector refresh, polygon ingest, alert loop
- server             ← http server, ws server, job scheduler
- app             ← user-facing “use-cases” (or “service”)
    - agent         
  - strategy
  - account
  - watchlist
  ...
- data            ← implementation of persistence & external I/O
  - postgres    ← db schema + queries (aggs, ticks, sectors…)
  - polygon
  - benzinga
  - cache       ← redis / in-mem






---
  .
├── cmd
│   ├── jobectl
│   │   └── main.go
│   ├── server
│   │   └── main.go
│   └── worker
│       └── main.go
├── Dockerfile.dev
├── Dockerfile.prod
├── go.mod
├── go.sum
├── internal
│   ├── app
│   │   ├── account
│   │   │   ├── tradeHandler.go
│   │   │   └── tradeStatistics.go
│   │   ├── agent
│   │   │   ├── backtestHelpers.go
│   │   │   ├── chat.go
│   │   │   ├── conversation.go
│   │   │   ├── executor.go
│   │   │   ├── gemini.go
│   │   │   ├── geminiHelpers.go
│   │   │   ├── persistentContext.go
│   │   │   ├── planner.go
│   │   │   ├── prompt.go
│   │   │   ├── prompts
│   │   │   │   ├── analyzeInstance.txt
│   │   │   │   ├── backtestSystemPrompt.txt
│   │   │   │   ├── defaultSystemPrompt.txt
│   │   │   │   ├── finalResponseSystemPrompt.txt
│   │   │   │   ├── initialQueriesPrompt.txt
│   │   │   │   ├── spec.txt
│   │   │   │   └── suggestedQueriesPrompt.txt
│   │   │   ├── query.go
│   │   │   ├── suggestions.go
│   │   │   └── tools.go
│   │   ├── alerts
│   │   │   └── alerts.go
│   │   ├── chart
│   │   │   ├── chart.go
│   │   │   ├── chartHelpers.go
│   │   │   ├── drawings.go
│   │   │   └── events.go
│   │   ├── exchange.go
│   │   ├── filings
│   │   │   └── edgar.go
│   │   ├── helpers
│   │   │   ├── exchange.go
│   │   │   └── security.go
│   │   ├── replay
│   │   │   ├── replay.go
│   │   │   └── ticks.go
│   │   ├── screensaver
│   │   │   └── screensaver.go
│   │   ├── settings
│   │   │   ├── authTools.go
│   │   │   └── settings.go
│   │   ├── strategy
│   │   │   ├── backtest.go
│   │   │   ├── backtestHelpers.go
│   │   │   ├── compile.go
│   │   │   ├── spec.go
│   │   │   └── strategies.go
│   │   ├── _study
│   │   │   └── study.go
│   │   └── watchlist
│   │       └── watchlist.go
│   ├── data
│   │   ├── conn.go
│   │   ├── edgar
│   │   │   ├── edgar.go
│   │   │   └── edgar_service.go
│   │   ├── polygon
│   │   │   ├── aggs.go
│   │   │   ├── market.go
│   │   │   ├── quote.go
│   │   │   ├── realtime.go
│   │   │   └── socket.go
│   │   ├── postgres
│   │   │   ├── exchange.go
│   │   │   └── security.go
│   │   ├── types.go
│   │   └── utils
│   │       ├── nullables.go
│   │       ├── task.go
│   │       └── time.go
│   ├── jobs
│   │   ├── alerts
│   │   │   ├── dispatch.go
│   │   │   ├── main.go
│   │   │   ├── news.go
│   │   │   ├── price.go
│   │   │   └── strategy.go
│   │   ├── daily_ohlcv.go
│   │   ├── email.go
│   │   ├── sectors.go
│   │   ├── securities.go
│   │   ├── securitiesTable.go
│   │   └── stream
│   │       └── polygon.go
│   └── server
│       ├── auth.go
│       ├── cli.go
│       ├── http.go
│       ├── schedule.go
│       └── socket.go
├── jobctl
├── README.md
└── tmp
    ├── build-errors.log
    └── main

