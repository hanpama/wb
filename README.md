# wb - Terminal Web Browser

A command-line web browser that renders web pages as markdown in your terminal. Browse websites, interact with forms, and navigate through pages - all from your CLI.

```
$ wb
[Status: 1 tabs open]
🌐 The active tab is [tab-1: "about:blank"].
Use 'wb show' to view its content or 'wb list' to see all tabs.

────────────────────────────────────────────────────────────────

Quick Start:
  1. Open a website:
     wb open https://example.com

  2. Interactive elements have {hash} identifiers:
     [Click here]{a1b2}  [Input/text: (empty)]{c3d4}

  3. Inspect an element to see what it does:
     wb describe a1b2

  4. Click links and buttons:
     wb click a1b2

  5. Fill in forms:
     wb input c3d4 "your text here"

────────────────────────────────────────────────────────────────
wb :: The TUI Web Browser

Usage:
  wb [flags]
  wb [command]

Available Commands:
  back        Navigate back in the current tab's history
  click       Click an interactive element by its hash
  close       Close the current or a specific tab
  completion  Generate the autocompletion script for the specified shell
  describe    Describe an interactive element by its hash
  enter       Press Enter on the currently focused element
  forward     Navigate forward in the current tab's history
  help        Help about any command
  list        List all open tabs
  new         Open a new tab with the specified URL
  open        Navigate the current tab to a different URL
  respond     Respond to a pending JavaScript dialog
  show        Show the current tab as markdown
  switch      Switch focus to a specific tab
  type        Type text into the currently focused element

Flags:
  -h, --help   help for wb

Use "wb [command] --help" for more information about a command.
```

```
$ wb open https://example.com
Example Domain
────────────────────────────────────────────────────────────────
# Example Domain

This domain is for use in documentation examples without needing permission. Avoid use in operations.

[Learn more]{3d805432}
────────────────────────────────────────────────────────────────
[Lines 1-5 / 5]
• tab-1 | https://example.com/
```