# wb Test Server

This test server provides various endpoints to test the wb terminal browser's new TUI improvements, specifically:

- Network request tracking
- WaitForStable functionality
- Conditional rendering (full vs diff)
- Network status footer display

## Quick Start

### 1. Start the Test Server

```bash
cd testserver
go run main.go
```

The server will start on `http://localhost:8080`

### 2. Build wb

In another terminal:

```bash
cd ..
go build -o wb
```

### 3. Run Tests

Open the test server home page:

```bash
./wb open http://localhost:8080
```

You'll see a list of test scenarios. Click through them to test different features.

## Test Scenarios

### 1. Instant Load (`/instant`)
Tests immediate rendering without network requests.
- **Goal**: Verify no network status footer appears
- **Flow**: [testflows/01-instant-load.md](testflows/01-instant-load.md)

### 2. Slow Resource Loading (`/slow-resource`)
Tests WaitForStable and network status tracking.
- **Goal**: Verify request counting and stability detection
- **Flow**: [testflows/02-slow-resources.md](testflows/02-slow-resources.md)

### 3. Form Interaction (`/form`)
Tests diff rendering for same-page updates.
- **Goal**: Verify diff render when URL doesn't change
- **Flow**: [testflows/03-form-interaction.md](testflows/03-form-interaction.md)

### 4. Click Navigation (`/navigation`)
Tests full rendering when URL changes.
- **Goal**: Verify full render on navigation
- **Flow**: [testflows/04-click-navigation.md](testflows/04-click-navigation.md)

### 5. SPA-style Updates (`/spa`)
Tests diff rendering for JavaScript DOM updates.
- **Goal**: Verify diff render for same-document changes
- **Flow**: [testflows/05-spa-updates.md](testflows/05-spa-updates.md)

### 6. Multiple Concurrent Requests (`/multiple-requests`)
Tests tracking of many simultaneous network requests.
- **Goal**: Verify accurate request counting
- **Flow**: [testflows/06-multiple-requests.md](testflows/06-multiple-requests.md)

## Test Endpoints

### Pages
- `GET /` - Home page with links to all test scenarios
- `GET /instant` - Page with no external resources
- `GET /slow-resource` - Page with slow-loading resources
- `GET /form` - Form submission test
- `GET /navigation` - Multi-page navigation test
- `GET /spa` - Single-page application style updates
- `GET /multiple-requests` - Many concurrent requests

### Utilities
- `GET /delay?seconds=N` - Returns JSON after N seconds delay (max 10)
- `GET /image.png` - Returns a 1x1 transparent PNG

## Key Features Being Tested

### Network Request Tracking
- ✅ Tracks in-flight requests via CDP events
- ✅ Filters by LoaderID (current navigation context)
- ✅ Ignores WebSocket connections
- ✅ Cleans up stale requests (30s timeout)
- ✅ Memory leak prevention (max 1000 requests per tab)

### WaitForStable
- ✅ Waits for active request count to reach 0
- ✅ Requires 500ms stable period
- ✅ 5-second timeout (practical approach)
- ✅ Context cancellation support

### Rendering Logic

#### Show Command
- Immediate snapshot (no waiting)
- Full render + network status
- Updates lastViewedSnapshots

#### Click/Input Commands
- Wait 100ms → WaitForStable
- Check URL change:
  - **Changed**: Full render + network status
  - **Unchanged**: Diff render
- Updates lastViewedSnapshots

#### Forward/Back Commands
- Wait 100ms → WaitForStable
- Always full render + network status
- Updates lastViewedSnapshots

### Network Status Footer
Shows when active requests > 0:
```
[⏳ 3 network requests in progress...]
```

## Debugging

### Enable Verbose Logging

Modify `testserver/main.go` to add request logging:

```go
http.HandleFunc("/delay", func(w http.ResponseWriter, r *http.Request) {
    log.Printf("Delay request: %s", r.URL.Query().Get("seconds"))
    handleDelay(w, r)
})
```

### Check wb Server Logs

```bash
tail -f /tmp/wb-server.log
```

### Manual Testing Tips

1. **Test network status timing**: Use `watch -n 1 './wb show'` to see request count update every second

2. **Test diff rendering**: Make small changes (click button) and verify only changed lines appear

3. **Test full rendering**: Navigate to new page and verify entire content is shown

4. **Test stability**: Open `/slow-resource` and run `show` multiple times to see count decrease

## Troubleshooting

### "Network requests in progress" never disappears
- Check if requests are actually completing: `curl http://localhost:8080/delay?seconds=1`
- Verify WaitForStable timeout (5s) is sufficient
- Check for infinite loops or polling in JavaScript

### Diff shows entire page instead of changes
- Verify URL actually stayed the same
- Check that lastViewedSnapshots is being updated
- Ensure markdown rendering is deterministic

### Request count seems wrong
- Verify LoaderID filtering is working (check tab.currentLoaderID)
- Check for orphaned requests from previous navigations
- Ensure cleanup goroutine is running

## Contributing Test Cases

To add a new test scenario:

1. Add endpoint handler in `main.go`
2. Add link on home page
3. Create test flow document in `testflows/NN-scenario-name.md`
4. Update this README

Follow the existing test flow format for consistency.
