# Node Bugs Dashboard

Pre-built JQL queries for the OpenShift Node team bug dashboard (Rich Filter `2775`, Jira filter `12398252`).

## Base Filter

All queries start with this base — it scopes to Node team bugs and excludes closed/obsolete:

```
(filter = "Node Components" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, "Won't Fix / Obsolete")
```

**Shorthand used below:** `<BASE>` refers to this clause.

## Filter Tabs

Each tab adds a clause to the base filter. Combine as: `<BASE> AND <tab-clause>`.

| Tab | JQL Clause |
|-----|-----------|
| Untriaged | `priority = Undefined OR "Release Blocker" = Proposed OR assignee in ("aos-node@redhat.com")` |
| Triaged | `status in (NEW, "To Do") AND priority not in (Undefined) AND ("Release Blocker" not in (Proposed) OR "Release Blocker" is EMPTY) AND assignee not in (unassigned_jira, "aos-node@redhat.com")` |
| Refined | `status in (ASSIGNED, POST, Modified) AND priority not in (Undefined)` |
| Verification Needed | `status in (ON_QA) AND priority not in (Undefined)` |
| Escape Analysis Needed | `("Customer Impact" = "Customer Escalated" OR "SFDC Cases Counter" > 0 OR issueFunction in linkedIssuesOfRemote(url, "https://access.redhat.com/support/cases/*")) AND status not in (NEW, "To Do", ASSIGNED) AND "Escape Reason" is EMPTY AND created >= 2025-01-01` |
| Blocker? | `"Release Blocker" = Proposed OR priority = Blocker AND "Release Blocker" is EMPTY` |
| Blocker+ | `cf[12319743] = Approved OR priority = Blocker` |
| Customer Issues | `"Customer Impact" = "Customer Escalated" OR "SFDC Cases Counter" > 0 OR issueFunction in linkedIssuesOfRemote(url, "https://access.redhat.com/support/cases/*")` |
| Escalations | `project = "Red Hat OpenShift Priority List" OR cf[12320844] = "Customer Escalated" OR labels in (shift_telco5g)` |
| ! CVE | `labels not in (SecurityTracking) AND issuetype not in (Vulnerability, Weakness)` |
| CVE | `labels in (SecurityTracking) OR issuetype in (Vulnerability, Weakness)` |
| Older than 100d | `created <= -100d` |
| My | `assignee = currentUser() OR watcher in (currentUser())` |
| Kueue | `Component in ("Node / Kueue")` |

**Note:** The "My" tab uses `currentUser()` which does NOT work with PATs. Replace with `assignee = "harpatil@redhat.com" OR watcher = "harpatil@redhat.com"`.

## Additional Modifiers

Combine these with any tab clause using `AND`:

### Version

```
AND fixVersion = "4.22.0"
```
```
AND fixVersion is EMPTY
```

### Open bugs only (exclude resolved)

```
AND status not in (Done, CLOSED, Obsolete, "Won't Fix / Obsolete", Verified)
```

### Including ON_QA in open

```
AND status not in (Done, CLOSED, Obsolete, "Won't Fix / Obsolete", Verified, ON_QA)
```

### Priority

Values: `Blocker`, `Critical`, `Major`, `Normal`, `Minor`, `Undefined`

```
AND priority = Critical
AND priority in (Blocker, Critical)
```

### Assignee

```
AND assignee = "harpatil@redhat.com"
```

### Age

```
AND created <= -30d
AND created >= -7d
AND updated <= -60d
```

### CVE due dates

```
AND due <= 7d
AND due <= 14d AND due > 7d
AND due <= 30d AND due > 14d
```

### Contract Priority

```
AND "Special Handling" in (contract-priority)
```

### Scrum Teams

```
AND filter = "Node Green Team"
AND filter = "Node Blue Team"
AND NOT (filter = "Node Green Team" OR filter = "Node Blue Team")
```

## Custom Fields Reference

| Field | ID | Values |
|-------|-----|--------|
| Release Blocker | `cf[12319743]` | `Proposed`, `Approved` |
| Customer Impact | `cf[12320844]` | `"Customer Escalated"` |
| SFDC Cases Counter | `cf[12320741]` | numeric (> 0 means linked cases) |
| Escape Reason | `cf[12321444]` | text field |
| Special Handling | `cf[12320040]` | `contract-priority` |

## User Lookup

To find a team member's Jira username from their display name, use the user search API:

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/user/search?username=First+Last" \
  | python3 -c "
import sys, json
for u in json.load(sys.stdin):
    print(f'{u[\"displayName\"]:<30} {u[\"name\"]}')"
```

Use the `name` field from the result in `assignee =` JQL clauses.

To list all current Node team assignees (derived from the base filter):

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) AND assignee is not EMPTY" \
  --data-urlencode "maxResults=200" \
  --data-urlencode "fields=assignee" \
  -G | python3 -c "
import sys, json
data = json.load(sys.stdin)
seen = {}
for issue in data['issues']:
    a = issue['fields'].get('assignee')
    if a and a['name'] not in seen:
        seen[a['name']] = a['displayName']
for name, display in sorted(seen.items(), key=lambda x: x[1]):
    print(f'{display:<30} {name}')"
```

To find your own Jira username:

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/auth/1/session" | python3 -c "import sys,json; print(json.load(sys.stdin)['name'])"
```

## Example Queries

### Untriaged blockers for 4.22

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND (priority = Undefined OR \"Release Blocker\" = Proposed OR assignee in (\"aos-node@redhat.com\")) AND fixVersion = \"4.22.0\" AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) ORDER BY priority DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,fixVersions" \
  -G | python3 -m json.tool
```

### Escalations assigned to a person

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND (project = \"Red Hat OpenShift Priority List\" OR cf[12320844] = \"Customer Escalated\" OR labels in (shift_telco5g)) AND assignee = \"harpatil@redhat.com\" AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) ORDER BY priority DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,fixVersions,labels" \
  -G | python3 -m json.tool
```

### Customer issues older than 100 days

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND (\"Customer Impact\" = \"Customer Escalated\" OR \"SFDC Cases Counter\" > 0 OR issueFunction in linkedIssuesOfRemote(url, \"https://access.redhat.com/support/cases/*\")) AND created <= -100d AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) ORDER BY created ASC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,created,fixVersions" \
  -G | python3 -m json.tool
```

### CVEs due next week

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND (labels in (SecurityTracking) OR issuetype in (Vulnerability, Weakness)) AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified, ON_QA) AND due <= 7d ORDER BY due ASC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,due,fixVersions" \
  -G | python3 -m json.tool
```

### Open bugs by version for a person

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND assignee = \"harpatil@redhat.com\" AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) ORDER BY fixVersion ASC, priority DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,priority,fixVersions" \
  -G | python3 -m json.tool
```

### Contract priority bugs with no fix version

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND \"Special Handling\" in (contract-priority) AND fixVersion is EMPTY AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified, ON_QA) ORDER BY priority DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,fixVersions" \
  -G | python3 -m json.tool
```

### Green team blocker+ bugs

```bash
JIRA_API_TOKEN=$(security find-generic-password -a "$USER" -s "JIRA_API_TOKEN" -w 2>/dev/null || secret-tool lookup service jira key JIRA_API_TOKEN) && \
curl -sL -H "Authorization: Bearer $JIRA_API_TOKEN" \
  "https://issues.redhat.com/rest/api/2/search" \
  --data-urlencode "jql=(filter = \"Node Components\" AND (project = OCPBUGS OR project = RHOCPPRIO) AND issueType in (Bug, Task, Vulnerability, Weakness) OR project = OCPNODE AND issueType = Bug) AND status not in (Obsolete, \"Won't Fix / Obsolete\") AND (cf[12319743] = Approved OR priority = Blocker) AND filter = \"Node Green Team\" AND status not in (Done, CLOSED, Obsolete, \"Won't Fix / Obsolete\", Verified) ORDER BY priority DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,status,assignee,priority,fixVersions" \
  -G | python3 -m json.tool
```
