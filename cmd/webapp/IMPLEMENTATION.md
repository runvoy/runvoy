# Webviewer Implementation Status

## Overview

Migration of the runvoy webviewer from a single-file HTML/JS application to a structured Svelte application for better maintainability and extensibility.

**Branch**: `2025-01-04-svelte-webviewer`
**Started**: 2025-01-04
**Current Status**: Initial setup phase

---

## Goals

1. **Improve maintainability**: Break monolithic 756-line HTML file into modular components
2. **Enable extensibility**: Make it easy to add CLI features to the web UI
3. **Maintain simplicity**: Keep single-file deployment model (static HTML with inlined CSS/JS)
4. **Preserve functionality**: All existing features must work identically

---

## Current Implementation (Legacy)

**File**: `cmd/webapp/dist/index.html` (WORK IN PROGRESS)

### Features
- View logs for a specific execution ID
- Real-time log streaming via WebSocket
- ANSI color code parsing and rendering
- Line numbers and timestamps display
- Authentication (API key + endpoint configuration)
- Local storage for credentials
- Retry logic for 404 errors (execution starting up)
- Log download functionality
- Toggle metadata display
- Auto-scroll to bottom
- Execution status display

### Key Components (in single file)
- Configuration management (API endpoint, API key)
- Execution state (ID, status, completion)
- Log fetching with retry (10s backoff, max 2 retries)
- WebSocket connection management
- ANSI parsing and rendering
- UI controls (play/pause, clear, download, metadata toggle)

### API Integration
- `GET /api/v1/executions/{id}/logs` - Fetch static logs + WebSocket URL
- `GET /api/v1/executions/{id}/status` - Get execution status
- WebSocket connection to `websocket_url` from logs response

### State Management
Global variables:
- `executionId` - Current execution being viewed
- `apiKey` - User's API key
- `API_ENDPOINT` - API base URL
- `logEvents` - Array of log events (sorted by timestamp)
- `websocket` - WebSocket connection instance
- `cachedWebSocketURL` - WebSocket URL from logs response
- `logsRetryCount` - Number of 404 retries attempted
- `isCompleted` - Whether execution has finished
- `isConnecting` - Whether WebSocket is connecting
- `showMetadata` - Toggle for line numbers/timestamps

---

## Target Implementation (Svelte)

### Project Structure

```
cmd/webapp/
├── package.json                 # Dependencies and build scripts
├── vite.config.js              # Vite build configuration
├── index.html                  # Entry point (minimal, loads app)
├── .gitignore                  # Node modules, build artifacts
├── IMPLEMENTATION.md           # This file
├── legacy-index.html           # Backup of original implementation
├── public/
│   └── favicon.ico
└── src/
    ├── main.js                 # App entry point
    ├── App.svelte              # Root component
    ├── components/
    │   ├── LogViewer.svelte           # Main log display area
    │   ├── StatusBar.svelte           # Execution status display
    │   ├── ConnectionManager.svelte   # API key/endpoint config modal
    │   ├── ExecutionSelector.svelte   # Input for execution ID
    │   ├── WebSocketStatus.svelte     # WebSocket connection indicator
    │   ├── LogControls.svelte         # Play/pause, clear, download buttons
    │   └── LogLine.svelte             # Individual log line with ANSI rendering
    ├── stores/
    │   ├── execution.js        # Execution ID, status, completion state
    │   ├── logs.js             # Log events, retry count
    │   ├── config.js           # API key, endpoint, localStorage sync
    │   └── websocket.js        # WebSocket connection state, cached URL
    ├── lib/
    │   ├── api.js              # API client (fetch wrapper)
    │   ├── ansi.js             # ANSI color code parser
    │   ├── websocket.js        # WebSocket connection manager
    │   └── utils.js            # Helper functions
    └── styles/
        └── global.css          # Global styles (Pico CSS + custom)
```

### Technology Stack

- **Svelte 4**: Component framework
- **Vite**: Build tool and dev server
- **Pico CSS**: Base styling (same as current)
- **vite-plugin-singlefile**: Bundle everything into single HTML file

### Component Responsibilities

#### `App.svelte`
- Root component
- Layout structure
- Route handling (URL query params)
- Initialize stores from localStorage

#### `LogViewer.svelte`
- Display log events in scrollable container
- Render log lines with metadata (line numbers, timestamps)
- Handle auto-scroll behavior
- Show loading/error states

#### `StatusBar.svelte`
- Display execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
- Show started time
- Color-coded status badges

#### `ConnectionManager.svelte`
- Modal for API key + endpoint configuration
- Save to localStorage
- Validate inputs
- Show/hide toggle

#### `ExecutionSelector.svelte`
- Input field for execution ID
- Load from URL query params
- Update URL when changed
- Trigger log fetching

#### `WebSocketStatus.svelte`
- Connection status indicator (connected/disconnected/connecting)
- Error messages
- Reconnect button

#### `LogControls.svelte`
- Play/Pause WebSocket connection
- Clear logs button
- Download logs button
- Toggle metadata display
- Show webviewer URL link

#### `LogLine.svelte`
- Render single log line
- Parse and display ANSI codes
- Show line number + timestamp (if metadata enabled)

### Store Design

#### `execution.js`
```javascript
import { writable } from 'svelte/store';

export const executionId = writable(null);
export const executionStatus = writable(null);
export const isCompleted = writable(false);
export const startedAt = writable(null);
```

#### `logs.js`
```javascript
import { writable } from 'svelte/store';

export const logEvents = writable([]);
export const logsRetryCount = writable(0);
export const showMetadata = writable(true);
```

#### `config.js`
```javascript
import { writable } from 'svelte/store';

// Synced with localStorage
export const apiEndpoint = writable(null);
export const apiKey = writable(null);
```

#### `websocket.js`
```javascript
import { writable } from 'svelte/store';

export const websocketConnection = writable(null);
export const cachedWebSocketURL = writable(null);
export const isConnecting = writable(false);
export const connectionError = writable(null);
```

### API Client (`lib/api.js`)

```javascript
class APIClient {
  constructor(endpoint, apiKey) {
    this.endpoint = endpoint;
    this.apiKey = apiKey;
  }

  async getLogs(executionId) { ... }
  async getStatus(executionId) { ... }
}

export default APIClient;
```

### Build Configuration

#### `vite.config.js`
```javascript
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  plugins: [
    svelte(),
    viteSingleFile()
  ],
  build: {
    target: 'esnext',
    assetsInlineLimit: 100000000,
    chunkSizeWarningLimit: 100000000,
    cssCodeSplit: false,
    outDir: 'dist',
    rollupOptions: {
      output: {
        inlineDynamicImports: true
      }
    }
  }
});
```

---

## Migration Plan

### Phase 1: Setup & Infrastructure ✅ COMPLETED
- [x] Create new branch `2025-01-04-svelte-webviewer`
- [x] Document current implementation
- [x] Backup legacy HTML file to `legacy-index.html`
- [x] Initialize package.json
- [x] Install dependencies (svelte, vite, vite-plugin-singlefile)
- [x] Create basic project structure (src/, components/, stores/, lib/, styles/)
- [x] Configure Vite for single-file output
- [x] Create new minimal index.html entry point

### Phase 2: Core Modules ✅ COMPLETED
- [x] Port ANSI parsing to `lib/ansi.js`
- [x] Create API client in `lib/api.js`
- [x] Implement Svelte stores (config, execution, logs, websocket)
- [x] Create global CSS with Pico + custom styles
- [x] Create minimal App.svelte shell
- [x] Create main.js entry point

### Phase 3: Components (MVP) ✅ COMPLETED
- [x] Build `App.svelte` shell
- [x] Build `ExecutionSelector.svelte`
- [x] Build `ConnectionManager.svelte`
- [x] Build `LogViewer.svelte` (basic)
- [x] Build `LogLine.svelte` with ANSI rendering

### Phase 4: WebSocket Integration ✅ COMPLETED
- [x] Port WebSocket logic to `lib/websocket.js`
- [x] Implement WebSocket store
- [x] Build `WebSocketStatus.svelte`
- [x] Integrate WebSocket with LogViewer

### Phase 5: Features & Polish ✅ COMPLETED
- [x] Build `StatusBar.svelte`
- [x] Build `LogControls.svelte`
- [x] Implement retry logic for 404s
- [x] Add download functionality
- [x] Add metadata toggle
- [x] Style with Pico CSS

### Phase 6: Testing & Validation 🚧 IN PROGRESS
- [ ] Test all features against legacy version
- [x] Test single-file build output
- [ ] Test in multiple browsers
- [ ] Verify localStorage persistence
- [ ] Test WebSocket reconnection

### Phase 7: Deployment
- [ ] Update build instructions
- [ ] Update deployment pipeline (if any)
- [ ] Replace legacy webviewer
- [ ] Archive legacy implementation

---

## Dependencies

```json
{
  "dependencies": {
    "svelte": "^4.0.0"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^3.0.0",
    "vite": "^5.0.0",
    "vite-plugin-singlefile": "^2.0.0"
  }
}
```

---

## Testing Checklist

When validating the Svelte implementation against legacy:

- [ ] Initial page load with execution ID in URL
- [ ] Initial page load without execution ID
- [ ] Enter API key + endpoint
- [ ] Fetch logs successfully
- [ ] Handle 404 with retry (10s, 10s, stop)
- [ ] Display logs with ANSI colors
- [ ] Line numbers increment correctly
- [ ] Timestamps display in UTC
- [ ] WebSocket connects automatically
- [ ] WebSocket receives and displays new logs
- [ ] Duplicate logs are filtered (by timestamp)
- [ ] Auto-scroll to bottom works
- [ ] Manual scroll disables auto-scroll
- [ ] Execution status updates
- [ ] Play/pause WebSocket connection
- [ ] Clear logs (local only)
- [ ] Download logs as text file
- [ ] Toggle metadata display
- [ ] Switch execution ID
- [ ] Browser back/forward navigation
- [ ] Credentials persist in localStorage
- [ ] Ctrl+C closes WebSocket gracefully
- [ ] Invalid API key shows error
- [ ] WebSocket disconnect/reconnect handling

---

## Progress Summary

**Last Updated**: 2025-01-04

### Completed
- ✅ Phase 1: Setup & Infrastructure
- ✅ Phase 2: Core Modules

### Current Status
- 🚧 Ready to start Phase 3: Components (MVP)
- Dev server can be started with `just local-dev-webapp`
- Minimal app shell loads and displays execution ID from URL

### Next Steps
1. Build ExecutionSelector component
2. Build ConnectionManager component
3. Build LogViewer component
4. Implement log fetching with retry logic
5. Add WebSocket integration

## Known Issues / TODOs

- None yet (infrastructure complete, ready for component development)

---

## Future Enhancements (Post-Migration)

Once Svelte migration is complete, these features become easier to add:

1. **Execution List View**
   - Display all executions
   - Filter by status, date, user
   - Click to view logs

2. **Kill Execution**
   - Button to stop running execution
   - Confirmation dialog

3. **User Management** (for admins)
   - List users
   - Create/revoke users
   - Display claim URLs

4. **Image Registry Management**
   - List registered images
   - Register new images
   - Set default image
   - Remove images

5. **Status Polling**
   - Real-time status updates for multiple executions
   - Notifications when execution completes

6. **Enhanced Filtering**
   - Search logs by keyword
   - Filter by log level (if applicable)
   - Regex search

7. **Dark Mode**
   - Toggle between light/dark themes
   - Persist preference

---

## Notes

- Keep deployment as simple as possible: single HTML file
- Maintain visual consistency with current implementation
- Prioritize feature parity before adding new features
- Document any deviations from legacy behavior
- Use this file to track progress and decisions
