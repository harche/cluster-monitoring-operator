---
name: redhat-support
description: Interact with Red Hat Jira (issues.redhat.com), search Red Hat Knowledge Base articles and solutions, and manage Customer Portal support cases. Use when the user asks about Jira issues, shares a Jira URL, asks about known issues, troubleshooting, knowledge base articles, support cases, shares a case/knowledge base URL, mentions issue keys like OCPBUGS-*, OCPNODE-*, any issues.redhat.com link, or asks about Node team bugs (untriaged, blockers, escalations, CVEs, customer issues, etc.).
allowed-tools: Bash(curl:*)
---

# Red Hat Support

Interact with Red Hat Jira, search the Knowledge Base, and manage Customer Portal support cases — all via REST APIs with `curl`.

## Authentication

### Jira (issues.redhat.com)

Uses a Bearer token from the `JIRA_API_TOKEN` environment variable:

```bash
curl -s -H "Authorization: Bearer $JIRA_API_TOKEN" "<url>" | python3 -m json.tool
```

**PAT limitation**: `currentUser()` in JQL and `/rest/api/2/myself` do NOT work with PATs on Red Hat Jira. To find the token owner's username:

```bash
# macOS
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w)
# Linux
JIRA_API_TOKEN=$(secret-tool lookup service jira key JIRA_API_TOKEN)

curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/auth/1/session" | python3 -m json.tool
```

Then use the `name` field (e.g., `harpatil@redhat.com`) in JQL queries like `assignee="harpatil@redhat.com"`.

### Knowledge Base & Support Cases

Both use the same two-step OAuth flow. Exchange the offline token for a short-lived access token:

```bash
ACCESS_TOKEN=$(curl -s https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token \
  -d grant_type=refresh_token -d client_id=rhsm-api \
  -d "refresh_token=$RH_API_OFFLINE_TOKEN" \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['access_token'])")
```

Always get a fresh access token at the start of each request sequence — tokens expire quickly. The offline token is available as the `RH_API_OFFLINE_TOKEN` environment variable.

## Quick Start

### Jira

```bash
# View an issue
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/issue/OCPNODE-4151?fields=summary,status,assignee,priority,issuetype,description,created,updated,components,labels,fixVersions" \
  | python3 -m json.tool

# Search with JQL
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search?jql=assignee%3D%22harpatil%40redhat.com%22+AND+type%3DEpic+AND+status+not+in+(Closed,Done)&maxResults=10&fields=summary,status,assignee,priority" \
  | python3 -m json.tool

# Get children of an epic
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search?jql=%22Epic+Link%22%3DOCPNODE-4151&fields=summary,status,assignee,issuetype,priority" \
  | python3 -m json.tool
```

### Knowledge Base

```bash
# Search the knowledge base
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=etcd+slow+fsync&rows=5" \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'Results: {data[\"response\"][\"numFound\"]}')
for doc in data['response']['docs']:
    print(f'  [{doc.get(\"documentKind\",\"\")}] {doc.get(\"publishedTitle\",\"\")[:70]}')
    print(f'    {doc.get(\"view_uri\",\"\")}')
"

# Fetch a specific article by ID
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=*:*&rows=1&fq=id:7087003&fl=publishedTitle,abstract,view_uri,issue,solution_resolution,solution_rootcause,solution_environment,solution_diagnosticsteps" \
  | python3 -m json.tool
```

### Support Cases

```bash
# View a specific case
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/04378910" \
  | python3 -m json.tool

# Search cases by keyword
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "keyword": "metallb"}' \
  | python3 -m json.tool
```

## Redirect Note

`issues.redhat.com` now 301-redirects to `redhat.atlassian.net` (Atlassian Cloud migration). All Jira `curl` calls must use `-L` (follow redirects). The Knowledge Base and Support Cases APIs are unaffected.

## URL Parsing

**Jira URLs** like `https://issues.redhat.com/browse/OCPNODE-4151` — extract the issue key (`OCPNODE-4151`) from the path after `/browse/`.

**Knowledge Base URLs** like `https://access.redhat.com/solutions/7087003` or `https://access.redhat.com/articles/6873281` — extract the numeric ID and fetch via `fq=id:{articleId}`.

**Case URLs** like `https://access.redhat.com/support/cases/#/case/04378910` — extract the case number (`04378910`) from the path after `/case/`. Case numbers are typically 8-digit strings and also appear in Jira bug SFDC case link fields.

**Note:** The legacy `/rs/solutions/{id}` API is decommissioned. Always use the search endpoint with `fq=id:{articleId}` to fetch individual knowledge base articles.

## References

Detailed command references — read on demand:

|references/jira.md — Issues, JQL search, epics, comments, transitions, custom fields
|references/node-bugs.md — Node team bug dashboard filters, version/assignee/priority queries, scrum teams
|references/knowledge-base.md — Query parameters, filters, field selection, document types, pagination
|references/cases.md — View, comment, attachments, case fields, search/filter, common queries

## Important

- **Always include clickable Jira URLs** when displaying issues, epics, or any Jira items. Format: `https://issues.redhat.com/browse/{KEY}` (e.g., `https://issues.redhat.com/browse/OCPBUGS-10431`).
- **Always confirm with the user before creating/updating Jira issues, adding comments, transitioning status, updating cases, or any write operation.**
- Jira uses a PAT from the OS secret store (`JIRA_API_TOKEN`). Knowledge base and support cases use OAuth (`RH_API_OFFLINE_TOKEN`).
- Always get a fresh OAuth access token before making knowledge base or case API calls.
- The Knowledge Base is **read-only**. Jira and support cases support both read and write operations.
- Use `fields=` parameter to limit Jira response size — full issue responses are very large.
- Epic children are found via JQL `"Epic Link"=EPIC-KEY`, NOT `parentEpic`.
- Use `fq=documentKind:Solution` to limit knowledge base results to solutions.
- All actions happen as the respective token owner.
