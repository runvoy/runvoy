# Runvoy Webviewer

Web-based log viewer for runvoy executions, built with Svelte.

## Development

```bash
# Install dependencies
npm install

# Start dev server (with hot reload)
npm run dev

# Build for production (single HTML file)
npm run build

# Preview production build
npm run preview
```

## Project Structure

```
cmd/webapp/
├── src/
│   ├── App.svelte              # Root component
│   ├── main.js                 # Entry point
│   ├── components/             # Svelte components
│   ├── stores/                 # Svelte stores (state management)
│   │   ├── config.js           # API endpoint, API key
│   │   ├── execution.js        # Execution ID, status
│   │   ├── logs.js             # Log events
│   │   └── websocket.js        # WebSocket connection
│   ├── lib/                    # Utilities and helpers
│   │   ├── api.js              # API client
│   │   └── ansi.js             # ANSI color parser
│   └── styles/
│       └── global.css          # Global styles (Pico CSS + custom)
├── legacy-index.html           # Original single-file implementation
├── IMPLEMENTATION.md           # Detailed migration docs
└── README.md                   # This file
```

## Build Output

The build process creates a single `dist/index.html` file with all CSS and JavaScript inlined. This can be served as a static file without any backend requirements.

## Migration Status

See [IMPLEMENTATION.md](./IMPLEMENTATION.md) for detailed migration progress and technical documentation.

**Current Status**: Phase 2 complete (infrastructure & core modules). Ready to build components.

## Legacy Implementation

The original single-file implementation is preserved as `legacy-index.html` for reference.
