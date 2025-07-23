import { writable } from 'svelte/store';

// Import the ContentChunk type from the chat interface
export type ContentChunk = { type: 'text' | 'table' | 'plot'; content: string | unknown };

export type TimelineEvent = {
  trigger: number; // 0→1
  forward: () => void;
  backward?: () => void;
  fired?: boolean;
};

export interface TimelineActions {
  addUserMessage: (text: string) => void;
  addAssistantMessage: (contentChunks: ContentChunk[]) => void;
  removeLastMessage: () => void;
  highlightEventForward: (rowIndex: number) => void;
  setChart: (keys: string[], chartType: 'candle' | 'line') => void;
}
export const sampleQuery = "Analyze the impact of tariffs on tech and retail stocks between 2018 and 2025.";
export const totalScroll = 5000; // px of wheel delta required for full timeline

// Global store containing the hero animation progress (0 → 1)
export const timelineProgress = writable(0);
// Helper to build a unique key for each chart slice (ticker + timestamp + timeframe)
function ChartKey(ticker: string, timestampMs: number, timeframe: string): string {
  return `${ticker}_${timestampMs}_${timeframe}`;
}
export function createTimelineEvents({
  addUserMessage,
  addAssistantMessage,
  removeLastMessage,
  highlightEventForward,
  setChart
}: TimelineActions): TimelineEvent[] {
  return [
    {
      trigger: 0,
      forward: () => addUserMessage(sampleQuery),
      backward: () => removeLastMessage()
    },
    {
      trigger: 0,
      forward: () => addAssistantMessage([
        {
          "type": "text",
          "content": "# Impact of 2018–2025 U.S. Tariff Events on Tech and Retail\n Here is an analysis of 6 tariff announcements and the following performance of $$QQQ-0$$ (tech proxy), $$WMT-0$$, and $$COST-0$$ (retail staples) over 1-week, 1-month, and 3-month horizons.\n"
        },
      ]),
      backward: () => removeLastMessage()
    },
    {
      trigger: 0.10,
      forward: () => addAssistantMessage([
        {
          "type": "table",
          "content": {
            "rows": [
              ["2018-03-01", "U.S. announces 25 % steel / 10 % aluminum tariffs (Section 232)", "-5.45%", "-3.96%", "-3.32%"],
              ["2019-05-10", "US hikes tariffs to 25 % on $200 B Chinese goods", "-1.00%", "5.51%", "3.98%"],
              ["2019-08-01", "US to levy 10 % on additional $300 B Chinese imports", "-2.37%", "4.81%", "6.37%"],
              ["2020-02-14", "US cuts tariffs on $120 B Chinese goods", "-22.38%", "1.16%", "-3.56%"],
              ["2025-01-16", "Trump signals 10 % universal / 60 % China tariffs", "5.12%", "13.67%", "14.82%"]
            ],
            "caption": "Price impact of each tariff event",
            "headers": [
              "Date",
              "Tariff Description",
              "1M % QQQ",
              "1M % WMT",
              "1M % COST"
            ]
          }
        }]),
      backward: () => removeLastMessage()
    },
    // Immediately after the table is injected, scroll it into view & focus first row
    {
      trigger: 0.105,
      forward: () => highlightEventForward(-1),   // scroll table only
      backward: () => {
        setChart([ChartKey("QQQ", 0, "1d")], "candle");
      }
    },
    {
      trigger: 0.20,
      forward: () => {
        highlightEventForward(2);
        setChart([ChartKey("QQQ", Date.UTC(2019, 8, 1), "1d"), ChartKey("WMT", Date.UTC(2019, 8, 1), "1d"), ChartKey("COST", Date.UTC(2019, 8, 1), "1d")], "line");
      }    // now highlight third row
    },
    {
      trigger: 0.30,
      forward: () => highlightEventForward(5),
      backward: () => highlightEventForward(2)
    },
    {
      trigger: 0.4,
      forward: () => {
        addAssistantMessage([
          {
            type: "plot",
            content: {
              "chart_type": "scatter",
              "title": "Stock Performance After Tariff Events",
              "data": [
                {
                  "x": [1, 2, 3, 4, 5],
                  "y": [1, 4, 2, 8, 5],
                  "type": "scatter",
                  "mode": "markers+lines",
                  "name": "Test Data"
                }
              ],
              "layout": {
                "xaxis": { "title": "X" },
                "yaxis": { "title": "Y" }
              }
            }
          }
        ]);
      },
      backward: () => removeLastMessage()
    },
    {
      trigger: 0.405,
      forward: () => highlightEventForward(-2), // -2 will scroll plot into view
      backward: () => { }
    },
    {
      trigger: 0.75,
      forward: () => addAssistantMessage([
        {
          type: "text",
          content: "## Key takeaways\n\n Tech showed larger swings: large downside in the early 2025 tariff threat; strong upside after China's Aug 23 2019 retaliation.\n\n• Retail was consistently resilient. $$WMT-0$$ posted positive 1- and 3-month returns in 5 of 6 events; $$COST-0$$ outperformed tech during the worst tech draw-downs (Feb 2020, Jan 2025).\n\n• Aug 23 2019 (China retaliation) delivered the best across-board results, suggesting markets had largely priced in U.S. escalation but viewed China's response as a catalyst for negotiation.\n"
        },
      ]),
      backward: () => removeLastMessage()
    },
  ];
} [

  {
    "type": "backtest_table",
    "content": {
      "caption": "Detailed instance-level backtest results",
      "columns": [
        "change_3m_pct",
        "event_date",
        "score",
        "timestamp",
        "entry_price",
        "change_1m_pct",
        "event_description",
        "ticker",
        "change_1w_pct"
      ],
      "strategyID": 12
    }
  },
  {
    "type": "backtest_plot",
    "content": {
      "length": 3,
      "plotID": 0,
      "chartType": "scatter",
      "chartTitle": "3-Month % Change After Each Tariff Event",
      "strategyID": 12,
      "xAxisTitle": "Event Date",
      "yAxisTitle": "3-Month % Change"
    }
  }
]
