# Runvoy Webviewer

Web-based log viewer for runvoy executions, built with SvelteKit.

## Development

```bash
# Install dependencies
npm install

# Start dev server (with hot reload)
npm run dev

# Build for production (static files)
npm run build

# Preview production build
npm run preview
```

## Project Structure

```text
cmd/webapp/
├── src/
│   ├── app.html                # SvelteKit app template
│   ├── routes/
│   │   ├── +layout.js          # Layout configuration (prerender)
│   │   └── +page.svelte        # Main page component
│   ├── components/             # Svelte components
│   ├── stores/                 # Svelte stores (state management)
│   │   ├── config.js           # API endpoint, API key
│   │   ├── execution.js        # Execution ID, status
│   │   ├── logs.js             # Log events
│   │   └── websocket.js        # WebSocket connection
│   ├── lib/                    # Utilities and helpers
│   │   ├── api.js              # API client
│   │   ├── websocket.js        # WebSocket connection logic
│   │   └── ansi.js             # ANSI color parser
│   └── styles/
│       └── global.css          # Global styles (Pico CSS + custom)
├── svelte.config.js            # SvelteKit configuration
├── vite.config.js              # Vite configuration for SvelteKit
├── legacy-index.html           # Original single-file implementation
└── README.md                   # This file
```

## Build Output

The build process creates a `dist/` directory containing:

- `index.html` - Main HTML file
- `_app/` - JavaScript and CSS assets

The output is optimized for static file hosting (e.g., S3). All routes are prerendered at build time.

## Deployment

The app is deployed to S3 using the `deploy-webapp` command in the justfile:

- Files are synced to `s3://bucket/webapp/`
- A copy of `index.html` is also available at `s3://bucket/webapp/index.html` for backward compatibility

## Technology Stack

- **SvelteKit** - Framework for building the application
- **adapter-static** - Static site generation adapter
- **Svelte 4** - UI framework
- **Vite** - Build tool (via SvelteKit)

## Legacy Implementation

The original single-file implementation is preserved as `legacy-index.html` for reference.
