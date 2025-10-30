# Test Flow 5: SPA-style Updates (Diff Render)

## Purpose
Test that JavaScript-based DOM updates without navigation show diff rendering.

## Endpoint
`http://localhost:8080/spa`

## Expected Behavior
- Click button → DOM changes → same URL → diff render
- Shows before/after difference in content
- No full page re-render

## Test Steps

1. Ensure test server is running

2. Open the SPA test page:
```bash
./wb open http://localhost:8080/spa
```

3. View initial state:
```bash
./wb show
```

Note the initial content: "Initial content. Click the button above to update."

4. Find the "Update Content" button hash:
```bash
./wb show | grep "Update Content"
```

Example: `[Update Content]{def45678}`

5. Click the button:
```bash
./wb click def45678
```

Expected: Diff output showing content change

6. Verify diff output shows:
```
- Initial content. Click the button above to update.
+ Updated content #1
+ Timestamp: 2025-10-20T...
```

7. Click the button again:
```bash
./wb click def45678
```

Expected: Diff shows counter increment (1 → 2)

## Expected Output
```
Diff Output:
- Updated content #1
+ Updated content #2
- Timestamp: 2025-10-20T12:34:56Z
+ Timestamp: 2025-10-20T12:34:58Z
```

## Success Criteria
✅ URL stays the same (documentChanged = false)
✅ Diff render triggered (not full render)
✅ Shows before/after differences
✅ Multiple clicks show incremental changes
✅ WaitForStable ensures DOM is settled
