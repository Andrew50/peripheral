export type TimelineEvent = {
  trigger: number; // 0→1
  forward: () => void;
  backward?: () => void;
  fired?: boolean;
};

export interface TimelineActions {
  addUserMessage: (text: string) => void;
  addAssistantMessage: (text: string) => void;
  removeLastMessage: () => void;
  updateChartHighlight: () => void;
  resetChartHighlight: () => void;
}
export const sampleQuery = "Analyze the impact of tariffs on tech and retail stocks over the last 10 years.";
export const totalScroll = 1500; // px of wheel delta required for full timeline
export function createTimelineEvents({
  addUserMessage,
  addAssistantMessage,
  removeLastMessage,
  updateChartHighlight,
  resetChartHighlight,
}: TimelineActions): TimelineEvent[] {
  return [      
    {
      trigger: 0,
      forward: () => addUserMessage(sampleQuery),
      backward: () => removeLastMessage()
    },
    {
      trigger: 0,
      forward: () => addAssistantMessage("Sure – let's break it down step-by-step."),
      backward: () => removeLastMessage()
    },
    {
      trigger: 0.5,
      forward: () => updateChartHighlight(),
      backward: () => resetChartHighlight()
    },
    {
      trigger: 0.75,
      forward: () => addAssistantMessage("Here's how that looks on the chart."),
      backward: () => removeLastMessage()
    }
  ];
} 