# Test Flow 4: Click Navigation (Full Render)

## Purpose
Test that clicking links that change URL triggers full page re-render.

## Endpoint
`http://localhost:8080/navigation`

## Expected Behavior
- Click link → URL changes → WaitForStable → Full render
- Network status shown if resources loading
- lastViewedSnapshots updated

## Test Steps

1. Ensure test server is running

2. Open the navigation page:
```bash
./wb open http://localhost:8080/navigation
```

Initially on page 1.

3. Show current page:
```bash
./wb show
```

Find the "Page 2" link hash, e.g., `{xyz98765}`

4. Click to navigate to Page 2:
```bash
./wb click xyz98765
```

Expected: Full render of Page 2 (not diff)

5. Verify the output shows full page:
```
[tab-1] Navigation Test - Page 2 | http://localhost:8080/navigation?page=2
────────────────────────────────────────────────────────────────

# Navigation Test - Page 2

Click links to navigate between pages. Each should trigger full re-render.

[Page 1]{...} | [Page 2]{...} | [Page 3]{...}

Current page content: 2

[Back to Home]{...}
```

6. Navigate to Page 3, verify full render again

7. Use browser back:
```bash
./wb back
```

Expected: Full render back to Page 2

## Success Criteria
✅ URL change detected (documentChanged = true)
✅ Full page rendered (not diff)
✅ WaitForStable called before snapshot
✅ Back/forward navigation works with full render
✅ Network status shown during loading
