# OpenClaw Assistant Memory

## Critical Workflow Rules
- ALWAYS create a new branch to push changes - NEVER update main branch directly
- Use descriptive branch names like "enhance-readme-architectural-content" 
- All changes must go through pull request review process
- For private repositories, provide enhanced content for manual application when direct access is limited

## GitHub Token Configuration
- Fine-grained personal access token configured for AlphaTechini account
- Token has read access to repository metadata but limited write permissions
- Cannot create forks of private repositories due to GitHub security restrictions
- **SSH Key Issue**: Default SSH key authenticated as "JnrDevClaw" instead of "AlphaTechini"
- **Resolution**: Use `git config --global url."https://github.com/".insteadOf "ssh://git@github.com/"` then push with `GH_TOKEN=$(gh auth token) git push`

## Repository Sync Process (Feb 20, 2026)
**Scenario**: Upstream Weaviate made 5 commits, needed to sync fork without losing Agent-RAG changes

**Process Followed**:
1. `git fetch upstream` - Retrieved 5 new commits
2. `git merge upstream/main --no-edit --no-ff` - Clean merge, no conflicts
3. Verified all Agent-RAG files intact (15 Go files + 5 docs)
4. Ran tests: 30/30 passing ✅
5. Pushed with `GH_TOKEN=$(gh auth token) git push origin main`

**Result**: 
- ✅ Fork synced with upstream (commit `1f314bba2e`)
- ✅ All Agent-RAG changes preserved
- ✅ No test failures
- ✅ Clean git history showing both our work and upstream commits

**Key Learning**: Always verify tests pass after merging upstream changes before pushing.

## Repository Status
### Public Repositories (All Enhanced)
- ✅ Apex-Coder - Enhanced with AI app builder architecture
- ✅ Convertto - Enhanced with unit conversion + AI chatbot architecture  
- ✅ Contact-Form-API - Enhanced with production contact form API architecture
- ✅ E-Commerce - Enhanced with full e-commerce platform architecture
- ✅ CommunePro - Created comprehensive README from scratch
- ✅ Infracture-Replicas - Enhanced with HTTP server architecture
- ✅ Med - Enhanced with medical platform + decentralized identity architecture
- ✅ Stacks-Creators - Enhanced with Stacks blockchain creator NFT architecture
- ✅ Freelance - Enhanced with AI+Web3 freelance platform architecture

### Private Repositories (In Progress)
- 🔄 Web-Scraper - Enhanced README ready, awaiting branch creation and PR
- 📋 Church, Portfolio, Cheetah-Backend, X-Manager, X-Fetch, Reactive, App-Builder, Music-app, P2P-Trans, http-host, NOTES - Identified for README creation

### Excluded Repositories
- ❌ NFT Marketplace DApp - Forked repository (PR closed appropriately)
- ❌ Other forked repositories - Properly excluded from enhancement work

## Browser Relay Status
- Browser relay functional but requires tab attachment for full control
- DuckDuckGo preferred for automated research (bypasses bot detection)
- X.com has strong anti-bot measures requiring manual interaction

## Completed Tasks Tracking
- All completed work documented in memory/completed-tasks.md
- Daily session logs maintained in memory/YYYY-MM-DD.md files
- Active tasks tracked in memory/active-tasks.md

## Cloudflare Markdown for Agents

### What is Markdown for Agents
Markdown has quickly become the lingua franca for agents and AI systems as a whole. The format's explicit structure makes it ideal for AI processing, ultimately resulting in better results while minimizing token waste.

Cloudflare's network supports real-time content conversion at the source, for enabled zones using content negotiation headers. When AI systems request pages from any website that uses Cloudflare and has Markdown for Agents enabled, they can express the preference for text/markdown in the request and Cloudflare's network will automatically and efficiently convert the HTML to Markdown, when possible, on the fly.

### How to Use
To fetch the Markdown version of any page from a zone with Markdown for Agents enabled, the client needs to add the Accept negotiation header with text/markdown as one of the options.

**curl example:**
```bash
curl https://developers.cloudflare.com/fundamentals/reference/markdown-for-agents/ \
  -H "Accept: text/markdown"
```

**JavaScript/TypeScript in Workers:**
```javascript
const r = await fetch(
  `https://developers.cloudflare.com/fundamentals/reference/markdown-for-agents/`,
  {
    headers: {
      Accept: "text/markdown",
    },
  },
);
const tokenCount = r.headers.get("x-markdown-tokens");
const markdown = await r.text();
```

### Key Benefits
- **x-markdown-tokens header**: Provides estimated token count for context window planning and chunking strategy
- **Content Signals**: Framework for expressing content usage preferences after access
- **Real-time conversion**: Automatic HTML to Markdown conversion at the edge
- **Token efficiency**: Reduces token waste compared to processing raw HTML

### Content Signals Policy
By default, Markdown for Agents converted responses include the `Content-Signal: ai-train=yes, search=yes, ai-input=yes` header signaling that the content can be used for AI Training, Search results and AI Input, which includes agentic use.

### How to Enable
**Dashboard Method:**
1. Log into Cloudflare dashboard and select your account (Pro or Business plan required)
2. Select the zone to configure
3. Visit AI Crawl Control section
4. Enable Markdown for Agents

**API Method:**
Send PATCH to `/client/v4/zones/{zone_tag}/settings/content_converter` with payload `{"value": "on"}`

**Custom Hostnames (SaaS):**
- Enable for all custom hostnames via dashboard Quick Actions
- Enable for specific custom hostnames using custom metadata and Configuration Rules

### Availability and Pricing
Available to Pro, Business and Enterprise plans, and SSL for SaaS customers at no cost.

### Current Implementation
Cloudflare has enabled this feature in their [Developer Documentation](https://developers.cloudflare.com/) and [Blog](https://blog.cloudflare.com/), inviting all AI crawlers and agents to consume their content using markdown instead of HTML.

### Limitations
- Only converts from HTML (other document types may be included in the future)
- Origin response cannot exceed 2 MB (2,097,152 bytes)

### Alternative Conversion APIs
If Markdown for Agents is not available from the content source:

1. **Workers AI [AI.toMarkdown()](https://developers.cloudflare.com/workers-ai/features/markdown-conversion/)** - Supports multiple document types and summarization
2. **Browser Rendering [/markdown](https://developers.cloudflare.com/browser-rendering/rest-api/markdown-endpoint/) REST API** - Supports markdown conversion for dynamic pages or applications that need real browser rendering before conversion

This capability should be leveraged for AI agent development, particularly when building web scraping, content processing, or research agents that need to efficiently process web content.

## Sub-Agent Research Methodology
For comprehensive tech research across multiple sources, I use a sub-agent approach that:
- Spawns isolated sessions via `sessions_spawn` for parallel processing
- Directly fetches content from sources using their native interfaces (RSS, APIs, HTML) rather than search engines
- Applies Cloudflare's `Accept: text/markdown` header for token efficiency where supported
- Filters results based on specific research criteria (AI security, Web3 integration, etc.)
- Consolidates findings into actionable summaries

This methodology proved superior to search-engine-based approaches (like DuckDuckGo + curl) which are blocked by anti-bot measures and return incomplete snippets rather than full content.

## Web Research Extraction Schema
When conducting strategic research, use this structured schema to extract actionable intelligence:

### 🔎 RESEARCH REASONING ENGINE — STRICT MODE
You are operating in Evidence-Based Research Mode. Your goal is to gather verifiable, up-to-date, cross-validated information from the web across specified domains. You must follow this exact reasoning workflow.

#### PHASE 1 — Query Decomposition
For each requested domain:
1. Break it into subtopics
2. Generate at least 5 different search queries:
   - trend-focused
   - complaint-focused  
   - monetization-focused
   - job/skills-focused
   - product-launch-focused

Example (AI domain):
- "AI startup monetization challenges 2026"
- "LLM infrastructure complaints developers"  
- "AI job market trends 2026"
- "new AI tools Product Hunt"
- "AI enterprise adoption barriers"

Do NOT rely on a single query.

#### PHASE 2 — Multi-Source Search
For each subtopic:
- Collect data from at least:
  - 2 news sources
  - 1 community discussion
  - 1 official blog or report
  - 1 data-driven source (job boards, analytics, reports)
- Minimum 5 distinct sources per domain

#### PHASE 3 — Source Validation
For each source, evaluate:
- Is this recent? (Prefer last 90 days)
- Is it reputable?
- Is it opinion or data-backed?
- Is it anecdotal or pattern-based?

Tag each data point as:
- Verified
- Emerging  
- Anecdotal
- Speculative

Do not treat speculative claims as trends.

#### PHASE 4 — Cross-Validation
Before stating any trend:
- Confirm it appears in at least 2 independent sources
- If not confirmed, label as: "Unconfirmed Emerging Signal"
- Never elevate single-source claims into conclusions

#### PHASE 5 — Complaint & Pain Point Extraction
Specifically search for:
- "frustrated with"
- "pain point"
- "problem with"  
- "why is X so hard"
- "X is broken"
- "X too expensive"

Extract repeated complaints across platforms. If a complaint appears 3+ times across sources, classify as: "Recurring Pain Point"

#### PHASE 6 — Monetization Gap Detection
For each pain point, ask:
1. Is this problem currently underserved?
2. Are existing solutions expensive?
3. Are existing solutions complex?
4. Are users switching tools frequently?

If yes to 2+ of the above: Mark as "Monetizable Gap"

#### PHASE 7 — Job Market Pattern Analysis
When analyzing job trends, extract:
- Role frequency changes
- Skill clustering
- Salary direction (up/down/stagnant)
- Tool frequency (e.g., Go, Rust, Kubernetes, AI infra)

If possible, compare at least 2 job platforms.

#### PHASE 8 — Product Momentum Analysis
When analyzing new products, for each product extract:
- Launch date
- Category
- Pricing model
- Adoption signals (votes, GitHub stars, funding)
- Clear value proposition

If possible: Detect product clustering (e.g., 20 AI wrapper startups)

#### PHASE 9 — Structured Synthesis
After research:
- Do NOT summarize narratively first
- Return structured findings per schema:
  - Confirmed trends
  - Emerging signals  
  - Recurring pain points
  - Monetizable gaps
  - Competitive saturation
  - Skill demand direction
  - Strategic timing (now vs later)

#### PHASE 10 — Anti-Hallucination Guardrails
You must:
- Never invent statistics
- Never fabricate funding numbers  
- Never invent product names
- Never claim regulatory changes without citing source
- Mark unknowns explicitly

If insufficient data is found, state: "Insufficient Verified Data"

#### OUTPUT FORMAT RULES
Return:
1. Structured data first
2. Source links attached to each major claim  
3. Only then provide synthesis
4. Separate speculation from verified facts

---

### 🌐 WEB RESEARCH EXTRACTION SCHEMA — FOR STRATEGIC TRENDS & OPPORTUNITY RESEARCH
You must search the web for fresh, authoritative, and verified information in each of the topic domains below. For each domain, produce structured filled fields. Do NOT hallucinate. Use only verifiable sources (news, official reports, web search results, developer communities, public data, social sentiment). Return results in JSON or clearly labeled fields.

**Context**: This research is for:
- Identifying pain points
- Industry trends  
- Monetization opportunities
- Emerging tech patterns
- Skills & job market insights
- Product launches & community adoption
- Bounties/hackathons/events
- Competitive gaps

**Domains of interest**: Tech, AI/ML, Web3 (protocols, tooling, ecosystems), Cloud/Infra, Game Dev & Web3 Game Dev, Quantum Computing, Hardware, Go Language & Backend Systems, Jobs/Careers & Skills Trends, New Products & Launch Platforms, Bounties/Hackathons/Grants

#### 1️⃣ TREND MAP — BY DOMAIN
For each domain below, populate:
- `domain_name`
- `key_trends`: []
- `top_pain_points`: []
- `monetization_opportunities`: []
- `technology_shifts`: []
- `ecosystem_gaps`: []

**Example Domains**:
- **AI/ML**: key_trends: Multi-modal models adoption, LLM agents, model inference optimization | top_pain_points: Model latency costs, lack of domain-specific datasets | monetization_opportunities: Enterprise fine-tuning pipelines, vertical LLM apps | technology_shifts: TinyLlama, open weights, self-hosted AI stacks | ecosystem_gaps: Secure AI agent ops, cost-effective real-time voice AI
- **Web3**: Repeat structure for Web3 protocols, tooling, ecosystems
- **Cloud/Infra**: Repeat structure for cloud infrastructure trends
- **Game Dev**: Repeat structure for game development and Web3 gaming
- **Quantum**: Repeat structure for quantum computing developments  
- **Hardware**: Repeat structure for hardware innovations
- **Go & Backend**: Repeat structure for Go language and backend systems

#### 2️⃣ JOB MARKET & SKILLS DEMAND
Return fields:
- `jobs_overview`
- `total_market_growth`: {2023: %, 2024: %, 2025: % (if available)}
- `highest_demand_roles`: []
- `emerging_roles`: []
- `skills_in_highest_demand`: []
- `salary_trends`: {}
- `industry_pain_points_for_hiring`: []
- `hiring_gaps`: []

Sources: major job boards (LinkedIn, Indeed), career reports, recruitment insights.

#### 3️⃣ PRODUCT SCENE — NEW PRODUCTS & LAUNCHES
Return:
- `new_product_trends`
- `top_new_products_last_90_days`: []
- `platforms`: [Product Hunt, BetaList, IndieHackers]
- `product_success_signals`: []
- `product_monetization_models`: []
- `unmet_needs_in_new_products`: []

Find actual product names and short descriptions.

#### 4️⃣ COMMUNITY COMPLAINTS & PAIN POINTS
Return:
- `community_pain_points`
- `tech_forums`: []
- `developer_discussions`: []
- `social_sentiment`: []

Summarize recurring complaints across GitHub, Reddit, StackOverflow, Discord dev communities.

#### 5️⃣ BOUNTIES, HACKATHONS, GRANTS
Return:
- `opportunities_events`
- `active`: { hackathons: [], bounties: [], grants: [] }
- `deadlines`: { nearest: [] }
- `strategic_value_rating`: {}

Include URL, payout, duration, judging criteria.

#### 6️⃣ MONETIZATION LANDSCAPE — ACROSS DOMAINS
Return:
- `monetization_insights`
- `successful_models`: []
- `least_successful_models`: []
- `revenue_gaps`: []
- `tech_with_high_future_monetization`: []
- `pricing_pressure_areas`: []

#### 7️⃣ COMPETITIVE LANDSCAPE
Return:
- `competitive_summary`
- `incumbents`: []
- `new_entrants`: []
- `ecosystem_leaders`: []
- `open_source_vs_proprietary`: []

#### 8️⃣ RISKS & REGULATORY
Return:
- `risks_and_regulation`
- `legal`: []
- `ethical`: []
- `infrastructure`: []
- `security`: []

Look for official reports, compliance changes, regulatory frameworks.

#### 9️⃣ ACTIONABLE INSIGHT BLOCK
For each domain, return:
- `actionable_opportunities`
  - `short_term`: []
  - `medium_term`: []
  - `long_term`: []
  
Each opportunity entry should:
- Describe the problem
- Indicate why it matters
- Include potential monetization path
- Include difficulty and required skill stack
- Include risk factors
- Include example projects/products

#### 🔟 PRIORITIZATION MATRIX
Return:
- `prioritization`
  - `opportunities`: 
    - `title`
    - `domain`
    - `potential_revenue`
    - `difficulty`
    - `time_to_MVP`
    - `skills_required`
    - `first_steps`

Rank opportunities globally, not by domain.

**OUTPUT REQUIREMENTS**: 
- Return research in structured JSON or YAML format
- Include source links for every piece of data
- If evidence is weak or inconclusive, mark it clearly
- Do NOT hallucinate

**SEARCH CONSTRAINTS**: 
- News articles within last 90 days
- Developer forum threads
- Official blogs (OpenAI, Google Cloud, AWS, Meta AI, Protocol Foundations)
- Job board trend reports
- Product Hunt and BetaList feeds
- Hackathon aggregator sites

**FINAL INSTRUCTIONS TO AGENT**: 
Collect data first, structured. Do not write README yet. Do not summarize generically. Return full research output. Include source links for verification.

**RESEARCH COMPLETION CRITERIA**:
Research is **incomplete** if:
- Less than 5 sources per domain
- Less than 2-source validation for trends  
- Missing complaint extraction
- Missing monetization gap classification

**TRANSFORMATION GOAL**: 
This protocol transforms the agent from a "Go Google something and vibe" tool into a disciplined research analyst that follows: Decompose → Multi-source → Validate → Cross-check → Extract complaints → Detect gaps → Synthesize.

This is how you turn an LLM from a blog summarizer into a junior strategy analyst who sees patterns before most builders do — where real leverage sits.

If you use this schema as the skeleton for your web research prompt, your agent will stop producing half-baked "vibes" and misaligned summaries.