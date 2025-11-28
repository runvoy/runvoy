<script lang="ts">
    import { isConnecting, connectionError, isConnected } from '../stores/websocket';
    import { isCompleted } from '../stores/execution';

    const statusText = $derived.by(() => {
        if ($isCompleted) return 'Execution finished';
        if ($isConnecting) return 'Connecting...';
        if ($isConnected) return 'Connected';
        if ($connectionError) return $connectionError;
        return 'Disconnected';
    });

    const statusClass = $derived.by(() => {
        if ($isCompleted) return 'status-completed';
        if ($isConnecting) return 'status-connecting';
        if ($isConnected) return 'status-connected';
        return 'status-disconnected';
    });
</script>

<div class="websocket-status {statusClass}">
    <span class="indicator"></span>
    <span>{statusText}</span>
</div>

<style>
    .websocket-status {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.5rem 1rem;
        border-radius: var(--pico-border-radius);
        font-size: 0.9em;
        margin-bottom: 1rem;
    }

    .indicator {
        display: inline-block;
        width: 10px;
        height: 10px;
        border-radius: 50%;
        background-color: #ccc;
    }

    /* Connecting */
    .status-connecting {
        background-color: #f3f3f3;
        color: #555;
    }
    .status-connecting .indicator {
        background-color: #f39c12;
        animation: pulse 1.5s infinite;
    }

    /* Connected */
    .status-connected {
        background-color: #e8f5e9;
        color: #2e7d32;
    }
    .status-connected .indicator {
        background-color: #4caf50;
    }

    /* Disconnected / Error */
    .status-disconnected {
        background-color: #ffebee;
        color: #c62828;
    }
    .status-disconnected .indicator {
        background-color: #f44336;
    }

    /* Completed */
    .status-completed {
        background-color: #e3f2fd;
        color: #1565c0;
    }
    .status-completed .indicator {
        background-color: #2196f3;
    }

    @keyframes pulse {
        0% {
            opacity: 1;
        }
        50% {
            opacity: 0.4;
        }
        100% {
            opacity: 1;
        }
    }
</style>
