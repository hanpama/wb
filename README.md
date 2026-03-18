# wb - Terminal Web Browser

A command-line web browser that renders web pages using the browser's accessibility tree. Browse websites, interact with forms, and navigate pages — all from your CLI.

## Quick Start

```bash
docker compose up -d
docker compose exec wb wb open https://example.com
```

```
Example Domain
────────────────────────────────────────────────────────────────
- heading "Example Domain" [level=1]
- paragraph
  - text "This domain is for use in documentation examples without needing permission."
- paragraph
  - link "Learn more" [url=https://iana.org/domains/example] {d8dca4ce}
────────────────────────────────────────────────────────────────
[Lines 1-5 / 5]
• tab-1 | https://example.com/
```

Pages are rendered as an accessibility tree (aria snapshot format). Interactive elements are marked with `{hash}` identifiers.

## Usage

```bash
wb open https://example.com       # Open a page
wb show                           # View current page
wb show --offset 100              # Scroll down
wb click {hash}                   # Click a link or button
wb type "search query"            # Type into focused input
wb enter                          # Press Enter (submit form)
wb describe {hash}                # Inspect element (AX info + DOM context)
wb eval "document.title"          # Execute JavaScript
wb back                           # Navigate back
wb forward                        # Navigate forward
wb list                           # List all tabs
wb new https://other.com          # Open new tab
wb switch tab-2                   # Switch tab
wb close                          # Close current tab
wb respond ok                     # Dismiss alert/confirm dialog
wb respond ok "text"              # Respond to prompt dialog
```

### Interacting with forms

```bash
wb click {hash}                   # Focus the input element
wb type "hello world"             # Type text
wb enter                          # Submit
```

### Inspecting elements

`describe` shows both accessibility info and actual DOM context:

```
Element {4813e715}
────────────────────────────────────────────────────────────────
  Role:     checkbox
  Name:     Cloud
  Checked:  false
────────────────────────────────────────────────────────────────
body {d7384484} > main {8c958ab7} > form {39ad32fc} > fieldset {3fe0383f}

<div class="relative inline-block mr-1.5 mb-1.5">
  ★ <input type="checkbox" name="c" value="cloud" id="cap-cloud" class="peer sr-only">
    <label for="cap-cloud"> Cloud ... {556e459a}
────────────────────────────────────────────────────────────────
```

Every element in the DOM context gets a `{hash}`, enabling chained `describe` calls to navigate the DOM tree freely.

## Docker

### Using the published image

```yaml
# docker-compose.yaml
services:
  wb:
    image: ghcr.io/hanpama/wb:main
    container_name: wb
    restart: unless-stopped
    shm_size: 2gb
    security_opt:
      - seccomp:unconfined
    deploy:
      resources:
        limits:
          memory: 4G
        reservations:
          memory: 1G
```

```bash
docker compose up -d
docker compose exec wb wb open https://example.com
```

### Building locally

```bash
docker compose up -d --build
```

## Architecture

wb uses Chrome's accessibility tree (via CDP `Accessibility.getFullAXTree`) instead of raw DOM parsing. This means:

- **Label association, name computation, role detection** are handled by the browser's accessibility engine — not reimplemented
- **Rendering** is a 2-phase pipeline:
  1. **Normalize**: flatten transparent nodes, remove noise, merge adjacent text
  2. **Render**: serialize the clean tree as indented text with `{hash}` markers
- **Interaction** uses `backendDOMNodeId` from AX nodes → CDP `DOM.resolveNode` → mouse/keyboard events
- **DOM inspection** (`describe`) bridges the AX view with actual HTML via CDP

## License

MIT
