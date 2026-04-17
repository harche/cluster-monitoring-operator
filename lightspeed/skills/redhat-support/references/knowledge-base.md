# Knowledge Base Search

Search the Red Hat Knowledge Base using the Hydra search API (Solr-based).

## Endpoint

```
GET https://access.redhat.com/hydra/rest/search/kcs
```

## Query Parameters

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Search query. Use `+` to join terms (AND). Use `*:*` to match all (when filtering by ID). |
| `rows` | int | Max results to return (default varies) |
| `start` | int | Offset for pagination |
| `fq` | string | Filter query. Can be repeated for multiple filters. |
| `fl` | string | Comma-separated field list to return |
| `sort` | string | Sort order (e.g., `lastModifiedDate desc`) |

## Filter Queries (fq)

Filter by document type:
```
fq=documentKind:Solution
fq=documentKind:Documentation
fq=documentKind:Article
```

Filter by ID (fetch specific article):
```
fq=id:7087003
fq=solution.id:6968450
```

Filter by product:
```
fq=boostProduct:openshift
```

Multiple filters (combine with separate `fq=` params):
```
fq=documentKind:Solution&fq=boostProduct:openshift
```

## Document Types

| Type | Description |
|---|---|
| `Solution` | Troubleshooting articles with issue/resolution/root cause |
| `Documentation` | Official product documentation sections |
| `Article` | General knowledge articles |
| `ContainerRepository` | Container image information |
| `Labs` | Red Hat Labs content |

## Field Selection (fl)

### Useful fields for search results

```
fl=publishedTitle,abstract,view_uri,documentKind,lastModifiedDate,id,boostProduct,boostDetectedVersion,caseCount
```

### Full solution fields (for individual article lookup)

```
fl=publishedTitle,abstract,view_uri,documentKind,lastModifiedDate,id,product,issue,solution_resolution,solution_rootcause,solution_environment,solution_diagnosticsteps,caseCount,category,component,internalTags
```

### All available fields

| Field | Description |
|---|---|
| `publishedTitle` | Article title |
| `abstract` | Short summary |
| `view_uri` | Web URL for the article |
| `documentKind` | Type (Solution, Documentation, Article, etc.) |
| `id` | Article ID |
| `lastModifiedDate` | ISO 8601 last modified date |
| `product` | Product name(s) (array) |
| `boostProduct` | Primary product (string) |
| `boostDetectedVersion` | Detected product version |
| `caseCount` | Number of linked support cases |
| `category` | Category tags (array) |
| `component` | Component tags (array) |
| `internalTags` | Internal tag list (array) |
| `issue` | Problem description (array, Solution only) |
| `solution_resolution` | Resolution steps (array, Solution only) |
| `solution_rootcause` | Root cause details (array, Solution only) |
| `solution_environment` | Affected environment (array, Solution only) |
| `solution_diagnosticsteps` | Diagnostic steps (array, Solution only) |
| `kcsState` | Article state (e.g., "verified") |
| `createdDate` | Creation date |

## Response Format

```json
{
  "responseHeader": { "QTime": 43 },
  "response": {
    "numFound": 148,
    "start": 0,
    "docs": [...]
  },
  "spellcheck": { "suggestions": [], "collations": [] }
}
```

## Common Queries

### Search Solutions Only

```bash
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=etcd+slow+fsync&rows=10&fq=documentKind:Solution&fl=publishedTitle,abstract,view_uri,id,lastModifiedDate,caseCount" \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'Solutions found: {data[\"response\"][\"numFound\"]}')
print()
for doc in data['response']['docs']:
    print(f'{doc[\"id\"]} - {doc.get(\"publishedTitle\",\"N/A\")}')
    print(f'  {doc.get(\"abstract\",\"N/A\")[:100]}')
    print(f'  URL: {doc.get(\"view_uri\",\"\")}')
    print(f'  Cases: {doc.get(\"caseCount\",0)} | Modified: {doc.get(\"lastModifiedDate\",\"\")}')
    print()
"
```

### Fetch Full Solution by ID

```bash
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=*:*&rows=1&fq=id:7087003&fl=publishedTitle,abstract,view_uri,id,product,issue,solution_resolution,solution_rootcause,solution_environment,solution_diagnosticsteps,lastModifiedDate,caseCount" \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
doc = data['response']['docs'][0]
print(f'Title: {doc[\"publishedTitle\"]}')
print(f'URL: {doc[\"view_uri\"]}')
print(f'Products: {\", \".join(doc.get(\"product\", []))}')
print(f'Cases: {doc.get(\"caseCount\", 0)}')
print(f'Modified: {doc.get(\"lastModifiedDate\", \"\")}')
print()
print('Environment:')
for e in doc.get('solution_environment', []):
    print(f'  {e}')
print()
print('Issue:')
for i in doc.get('issue', []):
    print(f'  {i}')
print()
print('Root Cause:')
for r in doc.get('solution_rootcause', []):
    print(f'  {r}')
print()
print('Diagnostic Steps:')
for d in doc.get('solution_diagnosticsteps', []):
    print(f'  {d[:200]}')
print()
print('Resolution:')
for r in doc.get('solution_resolution', []):
    print(f'  {r[:300]}')
"
```

### Search by Product

```bash
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=certificate+expired&rows=5&fq=documentKind:Solution&fq=boostProduct:openshift&fl=publishedTitle,view_uri,id,boostDetectedVersion,caseCount" \
  | python3 -m json.tool
```

### Recent Solutions (sorted by date)

```bash
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=openshift+upgrade&rows=10&fq=documentKind:Solution&sort=lastModifiedDate+desc&fl=publishedTitle,view_uri,id,lastModifiedDate,caseCount" \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
for doc in data['response']['docs']:
    print(f'{doc.get(\"lastModifiedDate\",\"\")[:10]} [{doc[\"id\"]}] {doc.get(\"publishedTitle\",\"\")[:60]}')
    print(f'  {doc.get(\"view_uri\",\"\")}')
"
```

### Pagination

```bash
# First page
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=upgrade+failure&rows=10&start=0&fq=documentKind:Solution" \
  | python3 -m json.tool

# Next page
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://access.redhat.com/hydra/rest/search/kcs?q=upgrade+failure&rows=10&start=10&fq=documentKind:Solution" \
  | python3 -m json.tool
```
