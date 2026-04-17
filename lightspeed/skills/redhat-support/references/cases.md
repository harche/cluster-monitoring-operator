# Support Cases

View, comment on, search, and manage Red Hat Customer Portal support cases.

## Base URL

```
https://api.access.redhat.com/support/v1
```

## View Case

```bash
# Full case details (includes comments)
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/{caseNumber}" \
  | python3 -m json.tool

# Extract key fields
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/{caseNumber}" \
  | python3 -c "
import sys, json
c = json.load(sys.stdin)
print(f'Case:     {c.get(\"uri\", \"\").split(\"/\")[-1]}')
print(f'Summary:  {c[\"summary\"]}')
print(f'Status:   {c[\"status\"]}')
print(f'Severity: {c[\"severity\"]}')
print(f'Product:  {c[\"product\"]} {c.get(\"version\", \"\")}')
print(f'Type:     {c.get(\"caseType\", \"N/A\")}')
print(f'Owner:    {c.get(\"ownerId\", \"Unassigned\")}')
print(f'Contact:  {c.get(\"contactName\", \"N/A\")}')
print(f'Created:  {c[\"createdDate\"]}')
print(f'Modified: {c[\"lastModifiedDate\"]}')
print(f'Escalated: {c.get(\"customerEscalation\", False)}')
print(f'CritSit:  {c.get(\"critSit\", False)}')
print()
print('Description:')
print(c.get('description', 'N/A')[:500])
"
```

## Case Fields

| Field | Description |
|---|---|
| `summary` | Case title |
| `description` | Detailed problem description |
| `status` | Current status (see Status Values below) |
| `severity` | Severity level (see Severity Values below) |
| `product` | Red Hat product name |
| `version` | Product version |
| `caseType` | Case type (e.g., "Defect / Bug", "Configuration Issue") |
| `ownerId` | Red Hat case owner name |
| `contactName` | Customer contact name |
| `contactSSOName` | Customer SSO username |
| `accountNumberRef` | Customer account number |
| `createdDate` | ISO 8601 creation timestamp |
| `lastModifiedDate` | ISO 8601 last modified timestamp |
| `createdById` | Creator display name |
| `lastModifiedById` | Last modifier display name |
| `customerEscalation` | Whether customer requested escalation (boolean) |
| `critSit` | Critical situation flag (boolean) |
| `requestManagementEscalation` | Management escalation requested (boolean) |
| `internalStatus` | Internal-facing status |
| `origin` | Case origin (e.g., "Web", "Phone") |
| `openshiftClusterID` | OpenShift cluster ID if applicable |
| `hotfixRequested` | Hotfix requested flag |
| `hotfixDelivered` | Hotfix delivered flag |
| `apiTags` | List of internal tags |

### Status Values

- `Waiting on Red Hat`
- `Waiting on Customer`
- `Closed`

### Severity Values

- `1 (Urgent)` — Production system down
- `2 (High)` — Significant business impact
- `3 (Normal)` — Standard issue
- `4 (Low)` — Minor issue or question

## Comments

```bash
# List all comments on a case
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/{caseNumber}/comments" \
  | python3 -m json.tool

# Format comments for readability
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/{caseNumber}/comments" \
  | python3 -c "
import sys, json
comments = json.load(sys.stdin)
for c in comments:
    visibility = c.get('visibility', 'Unknown')
    public = 'Public' if c.get('isPublic') else 'Internal'
    print(f'--- [{public}/{visibility}] {c[\"createdBy\"]} ({c[\"createdDate\"]}) ---')
    body = c.get('commentBody', '')
    print(body[:500])
    if len(body) > 500:
        print(f'... ({len(body)} chars total)')
    print()
"

# Add a comment (ALWAYS confirm with user first)
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/{caseNumber}/comments" \
  -d '{"commentBody": "Comment text here (max 32K characters)"}' \
  | python3 -m json.tool
```

### Comment Fields

| Field | Description |
|---|---|
| `commentBody` | Comment text content |
| `isPublic` | Whether visible to customer (boolean) |
| `visibility` | "Customer" or "Associate" |
| `createdBy` | Author display name |
| `createdDate` | ISO 8601 timestamp |
| `createdByType` | "Associate" or "Customer" |
| `caseNumber` | Parent case number |
| `id` | Comment ID |

## Attachments

```bash
# List attachments on a case
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://api.access.redhat.com/support/v1/cases/{caseNumber}/attachments" \
  | python3 -m json.tool
```

## Search & Filter

### Filter Endpoint

```bash
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{FILTER_JSON}' \
  | python3 -m json.tool
```

### Filter Parameters

| Parameter | Type | Description |
|---|---|---|
| `maxResults` | int | Max cases to return (default varies) |
| `offset` | int | Skip N cases for pagination |
| `keyword` | string | Text search across case summary and description |
| `status` | string | Filter by status (e.g., "Closed", "Waiting on Red Hat", "Waiting on Customer") |
| `product` | string | Filter by product name (e.g., "OpenShift Container Platform") |
| `startDate` | string | ISO 8601 datetime, cases created after this date |
| `endDate` | string | ISO 8601 datetime, cases created before this date |

### Common Queries

```bash
# List all open cases
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 20}' \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'Total open cases: {data[\"totalCount\"]}')
print(f'{\"Case\":<12} {\"Status\":<25} {\"Severity\":<15} {\"Summary\"}')
print('-' * 100)
for c in data['cases']:
    case_num = c.get('uri', '').split('/')[-1]
    print(f'{case_num:<12} {c[\"status\"]:<25} {c[\"severity\"]:<15} {c[\"summary\"][:50]}')
"

# Search by keyword
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "keyword": "search term here"}' \
  | python3 -m json.tool

# Filter by product
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "product": "OpenShift Container Platform"}' \
  | python3 -m json.tool

# Closed cases
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "status": "Closed"}' \
  | python3 -m json.tool

# Cases in date range
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "startDate": "2026-02-01T00:00:00Z", "endDate": "2026-02-28T23:59:59Z"}' \
  | python3 -m json.tool
```

### Pagination

```bash
# First page
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "offset": 0}' \
  | python3 -m json.tool

# Next page
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST "https://api.access.redhat.com/support/v1/cases/filter" \
  -d '{"maxResults": 10, "offset": 10}' \
  | python3 -m json.tool
```

## Multiple Cases from Jira

When Jira bugs reference SFDC case numbers (e.g., from `customfield_12313441`), look up each case:

```bash
for case_id in 04378910 04382468 04388043; do
  echo "=== Case $case_id ==="
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    "https://api.access.redhat.com/support/v1/cases/$case_id" \
    | python3 -c "
import sys, json
c = json.load(sys.stdin)
print(f'  Summary:  {c[\"summary\"]}')
print(f'  Status:   {c[\"status\"]}')
print(f'  Severity: {c[\"severity\"]}')
print(f'  Product:  {c[\"product\"]} {c.get(\"version\", \"\")}')
print(f'  Contact:  {c.get(\"contactName\", \"N/A\")}')
print(f'  Modified: {c[\"lastModifiedDate\"]}')
"
  echo
done
```
