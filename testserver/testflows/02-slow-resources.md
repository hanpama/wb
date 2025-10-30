# Test Flow 2: Slow Resource Loading

## Purpose
Test WaitForStable functionality and network status footer display.

## Endpoint
`http://localhost:8080/slow-resource`

## Expected Behavior
- Initial `show` displays network status footer
- Multiple `show` commands show decreasing request count
- After resources load, network status disappears

## Test Steps

1. Ensure test server is running

2. Open the page:
```bash
./wb open http://localhost:8080/slow-resource
```

3. Immediately check status (within 1 second):
```bash
./wb show
```

Expected: Should see "3 network requests in progress" (or similar)

4. Wait 1-2 seconds and check again:
```bash
./wb show
```

Expected: Should see fewer requests (1-2 remaining)

5. Wait 5 seconds total, then check:
```bash
./wb show
```

Expected: No network status footer

## Expected Output (First Show)
```
...
[⏳ 3 network requests in progress...]
────────────────────────────────────────────────────────────────
```

## Expected Output (After Stability)
```
...
────────────────────────────────────────────────────────────────
[Lines 1-15 / 15]
```

## Success Criteria
✅ Network status footer appears when resources are loading
✅ Request count decreases over time
✅ Footer disappears when all requests complete
✅ WaitForStable waits for 500ms stable period
