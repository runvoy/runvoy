<script lang="ts">
    interface Props {
        isConnecting: boolean;
        isConnected: boolean;
        connectionError: string | null;
        isCompleted: boolean;
    }

    const {
        isConnecting = false,
        isConnected = false,
        connectionError = null,
        isCompleted = false
    }: Props = $props();

    const statusText = $derived.by(() => {
        if (isCompleted) return 'Done';
        if (isConnecting) return 'Connecting';
        if (isConnected) return 'Live';
        if (connectionError) return 'Error';
        return 'Offline';
    });

    const statusClass = $derived.by(() => {
        if (isCompleted) return 'status-completed';
        if (isConnecting) return 'status-connecting';
        if (isConnected) return 'status-connected';
        return 'status-disconnected';
    });
</script>

<div class="websocket-status {statusClass}" title={connectionError || statusText}>
    <span class="indicator"></span>
    <span class="label">{statusText}</span>
</div>

<style>
    .websocket-status {
        display: inline-flex;
        align-items: center;
        gap: 0.375rem;
        padding: 0.125rem 0.5rem;
        border-radius: 0.75rem;
        font-size: 0.6875rem;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.02em;
    }

    .indicator {
        display: inline-block;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background-color: #ccc;
    }

    .label {
        line-height: 1;
    }

    /* Connecting */
    .status-connecting {
        background-color: rgba(243, 156, 18, 0.15);
        color: #f39c12;
    }
    .status-connecting .indicator {
        background-color: #f39c12;
        animation: pulse 1.5s infinite;
    }

    /* Connected */
    .status-connected {
        background-color: rgba(76, 175, 80, 0.15);
        color: #4caf50;
    }
    .status-connected .indicator {
        background-color: #4caf50;
    }

    /* Disconnected / Error */
    .status-disconnected {
        background-color: rgba(244, 67, 54, 0.15);
        color: #f44336;
    }
    .status-disconnected .indicator {
        background-color: #f44336;
    }

    /* Completed */
    .status-completed {
        background-color: rgba(33, 150, 243, 0.15);
        color: #2196f3;
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
