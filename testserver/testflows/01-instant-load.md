# Test Flow 1: Instant Load

## Purpose
Test that pages with no external resources render immediately without showing network status.

## Endpoint
`http://localhost:8080/instant`

## Expected Behavior
- `show` command returns immediately
- NO network status footer (no "X network requests in progress" message)
- Full page content visible

## Test Steps

1. Start test server:
```bash
cd testserver
go run main.go
```

2. In another terminal, open the page:
```bash
./wb open http://localhost:8080/instant
```

3. Verify immediate load:
```bash
./wb show
```

## Expected Output
```
[tab-1] Instant Load Test | http://localhost:8080/instant
────────────────────────────────────────────────────────────────

# Instant Load Test

This page has no external resources and should render immediately.

When you run 'wb show', you should NOT see any "network requests in progress" message.

[Back to Home]{abc12345}

────────────────────────────────────────────────────────────────
[Lines 1-10 / 10]
```

## Success Criteria
✅ Page renders completely
✅ No network status footer appears
✅ All content visible immediately
