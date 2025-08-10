package agent

// Suggestions guidance for planner direct-answer output
const suggestionsGuidelinesPlanner = `
**Suggested Follow-Up Questions:**
*   Generate 2-3 concise and relevant follow-up questions based on:
    *   The content you just provided in your response
    *   Natural follow-up topics the user might be interested in
    *   Opportunities to explore related analysis, data, or insights
    *   Questions that would make good use of available function tools
*   **Phrase them from the user's perspective** - It should not include phrases like "Would you like me to..." or "Should I...".
*   Do NOT reveal function names in suggestions
*   Focus on high-value, interesting follow-ups that extend the conversation meaningfully
`

// Suggestions guidance for final response prompt
const suggestionsGuidelinesFinal = `
<suggested_queries>
Generate 3 concise and relevant, high-value suggested follow-up questions. This should be based on:
- The content provided in your response
- Natural follow-up the user might be interested in
- Opportunities to explore related analysis, data, or insights
- Questions that would make good use of available function tools
- The suggested questions should be phrased from the perspective of the user asking the agent a question or to perform a task.
- It should NOT be framed from you (the agent's) perspective.
- It should not include phrases like "Would you like me to..., Do you want me to..." 
CRITICAL:
- Do NOT reveal function names in suggestions
- Do **NOT** use the special ticker formatting in the suggestions
</suggested_queries>
`
