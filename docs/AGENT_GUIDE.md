(Note: This project uses an AI "agent" as one of its core front-facing features for strategy generation and user assistance through a chat interface. This guide will clarify how the agent system works and how to extend or maintain it.)

Overview of the Agent System

The Peripheral Agent is an AI-driven component that serves as one of the platform's primary user-facing features, helping users in two ways:

Strategy Generation: Taking a natural language description of a strategy idea and turning it into working Python code (strategy function) using an AI model (o3 and gemini in our codebase).

Chat Assistant: Answering user questions about markets or the platform, and guiding them in using Peripheral through the integrated chat interface.

The agent system is primarily implemented in the backend service under internal/app/agent/:
planner.go – Orchestrates calls to the GPT model (Gemini) to get plan or code.
chat.go – Manages the conversational context and Q&A.
prompts/ – Contains prompt templates for various tasks (initial query prompt, system prompts with instructions for the AI, etc.).
How it works: When a user provides a prompt to create a strategy:
The backend calls our AI service (via API key) with a carefully crafted prompt (see prompts/spec.txt and others) that instructs the model on how to output code following our format

.
The AI returns Python code and possibly a description. The backend then validates this code with the security validator and either returns it to user or asks AI for corrections if it fails validation.
For the chat assistant:
We maintain context of the conversation in memory (or ephemeral storage). The conversation.go might keep a sliding window of last N messages.
The system prompt (see defaultSystemPrompt.txt) ensures the AI knows its role (e.g., "You are an expert trading assistant...").
When user asks something, we include relevant context (perhaps results from a strategy or data via the agent's tools) and get a response from GPT.
Workflows and Tips for Using/Extending the Agent
Strategy Generation Workflow:
User input (strategy description) -> API /api/strategies with a prompt.
Backend CreateStrategyFromPrompt logic (in strategies.go or agent) formulates an AI prompt:
Includes required function signature, example usage, and constraints (like "use pandas as pd, no forbidden imports").
The prompt likely references an example (the content of prompts/spec.txt includes instructions on output format).
AI model returns code.
Backend runs the code through validator.py in the worker or a dry-run to ensure it:
Contains required function name and parameters,
Uses allowed libraries,
Does not exceed limits.
If any issues, backend might append the error messages to the prompt and ask the AI to fix them (iterative).
Once validated, the strategy is saved and returned to user.
Guidance: If you need to update the strategy generation:
Update prompt templates in services/backend/internal/app/agent/prompts/. For example, if our requirements change (say we add a new required field in strategy output), update spec.txt and others accordingly.
Keep the Backend-Worker sync: The prompt’s specified function signature and output format must match what the worker expects


. If you change one, change the other.
For example, if we decide to rename timestamp field to date in results, we'd need to update:
System prompt spec (in agent prompt template),
Worker validator to allow date instead of timestamp,
Worker engine to output it,
Possibly frontend expectations.
Test changes by running a few prompts through the system (you can simulate by calling the backend function or via the API, using your OpenAI key in a dev environment).
Chat Assistant Workflow:
User enters a question in the chat UI -> frontend calls /api/chat with message.
Backend chat.go receives it, appends to conversation log (in memory or redis if we scale horizontally).
It crafts a prompt for GPT with system message + conversation history + maybe additional data.
Additional data: The agent might have tools (like it could fetch latest price or news to incorporate in answer). If so, the tools.go may define some pseudo-code for AI to request data, but currently, we often use a simpler approach where the AI is just providing answers based on its training and some provided context (like recent strategy results if question pertains to them).
The AI responds with an answer, possibly with references or suggestions.
Backend returns that to frontend, and the UI displays it. If streaming is enabled, the backend might stream the response chunk by chunk over the WebSocket or server-sent events.
Guidance for Chatbot:
The quality of responses depends heavily on the prompt. See prompts/defaultSystemPrompt.txt and others:
This likely includes instructions like "You are Peripheral AI assistant. You have knowledge of trading and can access user’s strategies. Do not provide financial advice, just informational...".
If the assistant should incorporate real data, we need to feed that in. For instance, if user asks "What's my best performing strategy?", the backend could fetch that info (maybe from past results) and prepend it to the AI prompt.
At the moment, it's unclear if we implemented such tooling. If not, improving the agent might include giving it the ability to call internal APIs (we could implement a tools system where the AI can request get_price(symbol) and the backend intercepts, executes it, and returns result for AI to continue).
If updating the chat agent, test with various questions. Ensure it doesn't violate any compliance (since it's internal, less worry of user-facing misinfo, but still).
Monitor token usage: Long conversations might hit token limits. We likely limit history to recent few interactions.
Automation Recipes
The agent is primarily user-facing through the chat interface and strategy generation features, but we might also have internal automation using it:
For example, an automatic strategy suggestion feature: the system might periodically analyze the user's portfolio and suggest a strategy via the chat.
Or using the agent for code explanations: user selects a piece of strategy code and the agent explains it (we could implement an endpoint where we send the code to GPT with "explain this code").
If implementing such features:
Reuse the agent infrastructure. Perhaps add new prompt templates or new functions in agent package to handle different tasks.
Keep prompts concise but informative.
Maintaining Sync (Agent, Validator, Examples)
As highlighted in the worker README, the agent's expectations and the worker's validation must be in lockstep


:
The system prompt defines what the AI should output (function name, parameters, return format)

.
The validator in services/worker/src/validator.py enforces those exact patterns


.
We also have example strategy implementations (examples.py) that demonstrate correct format

.
If you change any one of these (prompt, code template, or validation rules), update all of them. Otherwise, the AI might produce code that the validator rejects.
Common sync issues are listed in the worker README (like forgetting to update allowed field names in system prompt after changing them in code)

. Use that as a checklist when modifying agent or worker logic.
Testing the Agent System
In a dev environment, you can test:
Strategy generation: Use the API or even call CreateStrategyFromPrompt with sample queries. Check that the returned code passes validator.py (you can run validator.py standalone with a code string to see if it flags issues).
Chat responses: Start the backend with your OpenAI key in GEMINI_API_KEY, then use the front chat UI or curl the /api/chat endpoint. Ask something like "Generate a strategy for mean reversion on S&P 500". See if it goes to the strategy creation flow or just answers (depending on how we route such queries).
Evaluate the content for accuracy and appropriateness. Since this is a user-facing feature, we need to ensure responses are accurate, helpful, and appropriate for public use. We care about correctness of strategy code and not giving unbounded financial advice (disclaimers are essential for user-facing financial guidance).
Agent Configuration
The model (Gemini) API details likely stored in GEMINI_API_KEY or similar. If using OpenAI, that's the OpenAI API Key.
We might also configure model parameters (temperature, max tokens) in code or via env. If you need the agent to be more creative or more deterministic, adjust those in the API call.
If using a self-hosted model or different provider (maybe "Grok API"), ensure the request formatting in planner.go matches what that API expects.
Future Improvements
Tool Use: Implement a mechanism for the agent to retrieve up-to-date data during conversation (e.g., if user asks "What's the price of AAPL now?", the agent should fetch it rather than guess). This could be done by pre-processing the question and intercepting certain queries, or by enabling an "Agent Tool" where the assistant can output a special token to request a tool and the backend fulfills it (out of scope for now, but a known approach).
Knowledge Base: The agent could be augmented with knowledge of the user's data (like their past performance) by providing that in context or fine-tuning. Document any fine-tuning or additional knowledge injection if done.
Error Recovery: If the AI returns code that fails validation multiple times, maybe our system currently just gives up or returns an error. We could improve by trying a different approach or at least giving the user a meaningful message ("The AI couldn't create a valid strategy, try rephrasing").
Multi-turn Strategy Refinement: Allow user to chat with AI to refine a strategy. For instance, user: "Make it use 50-day MA instead", AI: "Okay, updated code: ...". This requires tying chat and strategy generation together; currently they might be separate flows.
This guide ensures developers understand the interplay between our AI agent and the rest of the system, and how to maintain and extend it responsibly. Always test agent changes thoroughly, as they directly impact the user experience and can affect user trust (we promise certain capabilities and must deliver correct output, especially for code generation where mistakes could cost money if a strategy behaves incorrectly). Keep the agent helpful, accurate, and aligned with our platform's purpose – empowering traders with AI through our core chat and strategy generation features, not misleading them.
/docs/adr/2023-01-10-database-choice.md (Example ADR)
Title: Database Choice – PostgreSQL/TimescaleDB vs NoSQL for Time-Series Data
Date: 2023-01-10
Status: Accepted Context:
The Peripheral platform needs to store large volumes of financial data (e.g., historical price data, fundamental metrics) as well as relational data (user accounts, strategies, etc.). We considered using a NoSQL or specialized time-series database to handle the time-series data (like InfluxDB or MongoDB for flexibility) separate from relational data. Decision:
Use a single PostgreSQL database with TimescaleDB extension for time-series data. Key Rationale:
Relational Queries: Many operations need joins between time-series data and relational data (e.g., fetch user’s strategies and related performance metrics from historical results). Using one Postgres DB allows straightforward JOINs and transactions across all data.
TimescaleDB Benefits: TimescaleDB (an extension to Postgres) provides automated table partitioning, compression, and efficient time-series queries

, giving near-specialized performance while staying in the Postgres ecosystem.
ACID and Consistency: Financial data is critical; Postgres offers strong ACID compliance. NoSQL stores might sacrifice consistency which is not acceptable for, say, account balances or precise trade records.
Team Familiarity: Our team has strong SQL expertise. Sticking to Postgres avoids a learning curve and maintenance overhead of an additional DB technology.
Simpler Infrastructure: Running one primary database service is simpler (backup, replication, monitoring) than integrating multiple database systems for different data types.
Considered Alternatives:
InfluxDB or Timescale (as separate service): Would handle time-series well, but we'd still need Postgres for relational data. It adds complexity and duplication (two backup systems, data synchronization or cross-service querying via app logic).
MongoDB: Flexible schema, could store all data in one DB. However, time-series in Mongo prior to their new time-series collections was not very efficient. Also, joining data (though possible via aggregation frameworks) is more complex than SQL. The strict schema of SQL actually helps maintain data integrity for our use-case.
Use Both SQL and NoSQL: E.g., Postgres for accounts and strategies, and a NoSQL for raw market data. Decided against due to complexity and the fact that Timescale in Postgres gave us needed performance.
Consequences:
We will manage large tables in Postgres; Timescale will automatically chunk them by time which should keep performance in check. We must be mindful of proper indexing (e.g., on time and symbol) for query speed.
All team members will continue writing SQL (which is fine). We will use an ORM minimally (perhaps for basic CRUD) but heavy queries might be hand-written in SQL for performance.
We set up robust backup (pg_dump and WAL archiving) and monitoring on the Postgres instance (see Backup ADR or docs) to ensure reliability given it's a single point of failure.
If down the line data volume grows beyond what one DB instance can handle (millions of rows per day), we might consider sharding or moving some seldom-used data to cold storage.
References:
TimescaleDB documentation on performance for billions of rows

.
Internal load testing results (see Appendix) showing Postgres with Timescale handling 100 million price rows with sub-second query times on standard hardware.
Discussion in engineering meeting on 2023-01-05 (decision to prioritize consistency over polyglot persistence).
(The above ADR is an example; refer to the ADR directory for additional records on other decisions.)