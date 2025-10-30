# Test Flow 6: Multiple Concurrent Requests

## Purpose
Test that the system correctly tracks multiple concurrent network requests and shows accurate count.

## Endpoint
`http://localhost:8080/multiple-requests`

## Expected Behavior
- Page starts 10 concurrent requests with delays 1-5 seconds
- `show` command displays accurate request count
- Request count decreases as requests complete
- Footer disappears when all complete

## Test Steps

1. Ensure test server is running

2. Open the page (this starts all requests):
```bash
./wb open http://localhost:8080/multiple-requests
```

The `open` command will wait for stability, so initial requests may have completed.

3. Immediately after, check status multiple times:
```bash
./wb show
```

Expected: "X network requests in progress" where X ≤ 10

4. Wait 2 seconds, check again:
```bash
sleep 2 && ./wb show
```

Expected: Fewer requests (requests with 1-2s delays completed)

5. Wait 4 seconds total, check again:
```bash
sleep 2 && ./wb show
```

Expected: Even fewer requests

6. Wait 6 seconds total, check final state:
```bash
sleep 2 && ./wb show
```

Expected: No network status footer (all requests complete)

## Timeline
- **T+0s**: 10 requests start (delays: 1,1,2,2,3,3,4,4,5,5)
- **T+1s**: 2 requests complete (8 remaining)
- **T+2s**: 2 more complete (6 remaining)
- **T+3s**: 2 more complete (4 remaining)
- **T+4s**: 2 more complete (2 remaining)
- **T+5s**: Last 2 complete (0 remaining)

## Expected Output (T+0s)
```
...
[⏳ 10 network requests in progress...]
```

## Expected Output (T+3s)
```
...
[⏳ 4 network requests in progress...]
```

## Expected Output (T+6s)
```
...
────────────────────────────────────────────────────────────────
[Lines 1-12 / 12]
```

## Success Criteria
✅ All 10 requests tracked correctly
✅ Request count decreases as expected over time
✅ LoaderID filtering works (only current navigation requests counted)
✅ Footer disappears when count reaches 0
✅ No memory leaks (requests cleaned up properly)
