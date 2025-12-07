/**
 * Types for execution actions
 */

export interface KillState {
    /** Whether a kill request is currently in-flight */
    isKilling: boolean;
    /** Whether kill was successfully initiated (prevents double-clicks) */
    killInitiated: boolean;
    /** Error message from last kill attempt, null if none */
    error: string | null;
}
