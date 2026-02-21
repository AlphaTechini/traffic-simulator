# Research Strategy: Exa vs DuckDuckGo

**Created:** February 21, 2026  
**Purpose:** Define when to use Exa API (serious research) vs DuckDuckGo (quick lookups)

---

## 🎯 EXA API - For Serious Research

**API Key:** `a556b0e1-8289-4c86-92c1-24385748b30c`

### Use Exa For:
- ✅ **Morning reports** (daily tech trends, opportunities)
- ✅ **Architecture case studies** (find real-world examples)
- ✅ **Technology validation** (find companies using X at scale)
- ✅ **Expert discovery** (find engineers with specific expertise)
- ✅ **Company research** (competitive analysis, tech stacks)
- ✅ **Academic papers** (research paper category)
- ✅ **News analysis** (recent developments, breaking news)
- ✅ **People search** (find experts by role/expertise)

### Endpoints:
- `/search` - General search with categories
- `/contents` - Extract full text from URLs (20K chars)
- `/answer` - Q&A with citations

### Categories:
| Category | Use Case | Example Query |
|----------|----------|---------------|
| `people` | Find engineers/architects | "software engineer distributed systems" |
| `company` | Find companies using tech | "AI startup healthcare" |
| `news` | Recent announcements | "OpenAI announcements" maxAgeHours: 24 |
| `research paper` | Academic papers | "transformer architecture improvements" |
| `tweet` | Real-time discussions | "AI safety discussion" |

### Configuration Best Practices:
```json
{
  "type": "auto",              // Balanced relevance/speed (default)
  "num_results": 10,           // Adjust based on need
  "maxAgeHours": 24,           // Daily-fresh content for news
  "contents": {
    "text": {
      "max_characters": 20000  // Full extraction
    }
  }
}
```

### Morning Report Workflow:
1. **Search tech trends:** `"AI infrastructure developments"` category: `news`, maxAgeHours: 24
2. **Find case studies:** `"scaling microservices at 100M users"` + engineering blogs
3. **Validate opportunities:** `"vector database startup funding"` category: `company`
4. **Extract full articles:** `/contents` endpoint for deep analysis
5. **Generate summary:** Use `/answer` endpoint for citation-backed summaries

---

## 🦆 DUCKDUCKGO - For Quick/Less Serious Research

### Use DuckDuckGo For:
- ✅ Quick fact-checking
- ✅ Simple how-to queries
- ✅ Documentation lookups
- ✅ General knowledge questions
- ✅ When speed > depth
- ✅ Non-critical research

### When NOT to Use:
- ❌ Morning reports (need depth + citations)
- ❌ Architecture validation (need case studies)
- ❌ Opportunity detection (need structured data)
- ❌ Competitive analysis (need company/people search)

---

## Decision Matrix

| Research Type | Tool | Why |
|--------------|------|-----|
| **Morning report** | Exa | Citations, full text, structured |
| **Tech trends** | Exa | News category, freshness control |
| **Case studies** | Exa | Full text extraction |
| **Expert finding** | Exa | People category |
| **Company research** | Exa | Company category |
| **Quick fact check** | DuckDuckGo | Fast, simple |
| **Documentation** | DuckDuckGo | Direct links |
| **General knowledge** | DuckDuckGo | Broad coverage |
| **Code snippets** | DuckDuckGo | GitHub/StackOverflow results |

---

## Integration Points

### System Design Visualizer
**File:** `internal/research/exa.go`

**Functions:**
- `FindCaseStudies(useCase, scale, technologies)` - Auto-find relevant examples
- `FindSimilarCompanies(techStack, industry)` - Competitive intelligence
- `FindExperts(expertise, technologies)` - Team planning
- `ValidateTechnologyChoice(technology, useCase, scale)` - Real-world validation

**Usage in AI Prompts:**
```go
// Before recommending architecture
caseStudies, _ := exa.FindCaseStudies("microservices", "100M users", []string{"Kafka", "PostgreSQL"})

// Include in AI response:
"Relevant case study: Instagram scaled PostgreSQL to billions of rows with partitioning
Source: https://instagram-engineering.com/..."
```

### Morning Reports (Daily Automation)
**Cron Job Schedule:** 5:00 AM daily

**Workflow:**
1. Search trending topics (news category, maxAgeHours: 24)
2. Find funding/acquisitions (company category)
3. Extract full articles (contents endpoint)
4. Generate summary with citations (answer endpoint)
5. Deliver via Telegram/Email

**Example Query Set:**
```json
[
  {"query": "AI infrastructure developments", "category": "news", "maxAgeHours": 24},
  {"query": "vector database startup funding", "category": "company"},
  {"query": "LLM agent production deployment", "category": "research paper"},
  {"query": "CTO engineer distributed systems", "category": "people"}
]
```

---

## Code Examples

### Exa Search (Go)
```go
exa := research.NewExaClient("a556b0e1-8289-4c86-92c1-24385748b30c")

// Find people
experts, _ := exa.SearchPeople("software engineer distributed systems", 10)

// Find companies
companies, _ := exa.SearchCompanies("AI startup healthcare", 10)

// Extract full text
contents, _ := exa.GetContents([]string{"https://example.com/article"}, 20000)

// Q&A with citations
answer, _ := exa.AnswerQuestion("Latest developments in quantum computing?", 5)
```

### Exa Search (Python - for scripts)
```python
from exa_py import Exa

exa = Exa(api_key="a556b0e1-8289-4c86-92c1-24385748b30c")

# People search
results = exa.search(
    query="software engineer distributed systems",
    category="people",
    num_results=10
)

# Company search
companies = exa.search(
    query="AI startup healthcare",
    category="company",
    num_results=10,
    contents={"text": {"max_characters": 20000}}
)
```

---

## Troubleshooting

### Results Not Relevant?
1. Try `type: "auto"` - most balanced option
2. Refine query - use singular form, be specific
3. Check category matches your use case

### Results Too Slow?
1. Use `type: "fast"`
2. Reduce `num_results`
3. Skip contents if you only need URLs

### No Results?
1. Remove filters (date, domain restrictions)
2. Simplify query
3. Try `type: "auto"` - has fallback mechanisms

---

## Resources

- **Docs:** https://exa.ai/docs
- **Dashboard:** https://dashboard.exa.ai
- **API Status:** https://status.exa.ai
- **MCP Server:** https://mcp.exa.ai/mcp?exaApiKey=YOUR_KEY

---

**Last Updated:** February 21, 2026  
**Maintained By:** AlphaTechini  
**Status:** ACTIVE - Used for all morning reports and serious research
