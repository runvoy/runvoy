/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import WebSocketStatus from './WebSocketStatus.svelte';
import { isConnecting, connectionError, isConnected } from '../stores/websocket';
import { isCompleted } from '../stores/execution';

describe('WebSocketStatus', () => {
    beforeEach(() => {
        isConnecting.set(false);
        connectionError.set(null);
        isConnected.set(false);
        isCompleted.set(false);
    });

    afterEach(() => {
        cleanup();
        isConnecting.set(false);
        connectionError.set(null);
        isConnected.set(false);
        isCompleted.set(false);
    });

    it('should display "Execution finished" when execution is completed', () => {
        isCompleted.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Execution finished')).toBeInTheDocument();
    });

    it('should display "Connecting..." when connecting', () => {
        isConnecting.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Connecting...')).toBeInTheDocument();
    });

    it('should display "Connected" when connected', () => {
        isConnected.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Connected')).toBeInTheDocument();
    });

    it('should display connection error when error is set', () => {
        connectionError.set('Connection failed');

        render(WebSocketStatus);

        expect(screen.getByText('Connection failed')).toBeInTheDocument();
    });

    it('should display "Disconnected" when not connected, not connecting, and no error', () => {
        isConnecting.set(false);
        isConnected.set(false);
        connectionError.set(null);
        isCompleted.set(false);

        render(WebSocketStatus);

        expect(screen.getByText('Disconnected')).toBeInTheDocument();
    });

    it('should prioritize completed status over connecting status', () => {
        isCompleted.set(true);
        isConnecting.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Execution finished')).toBeInTheDocument();
        expect(screen.queryByText('Connecting...')).not.toBeInTheDocument();
    });

    it('should prioritize completed status over connected status', () => {
        isCompleted.set(true);
        isConnected.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Execution finished')).toBeInTheDocument();
        expect(screen.queryByText('Connected')).not.toBeInTheDocument();
    });

    it('should prioritize connecting status over connected status', () => {
        isConnecting.set(true);
        isConnected.set(true);

        render(WebSocketStatus);

        expect(screen.getByText('Connecting...')).toBeInTheDocument();
        expect(screen.queryByText('Connected')).not.toBeInTheDocument();
    });

    it('should prioritize connecting status over error status', () => {
        isConnecting.set(true);
        connectionError.set('Some error');

        render(WebSocketStatus);

        expect(screen.getByText('Connecting...')).toBeInTheDocument();
        expect(screen.queryByText('Some error')).not.toBeInTheDocument();
    });

    it('should apply status-completed class when execution is completed', () => {
        isCompleted.set(true);

        const { container } = render(WebSocketStatus);

        const statusElement = container.querySelector('.status-completed');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-connecting class when connecting', () => {
        isConnecting.set(true);

        const { container } = render(WebSocketStatus);

        const statusElement = container.querySelector('.status-connecting');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-connected class when connected', () => {
        isConnected.set(true);

        const { container } = render(WebSocketStatus);

        const statusElement = container.querySelector('.status-connected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-disconnected class when disconnected', () => {
        isConnecting.set(false);
        isConnected.set(false);
        connectionError.set(null);
        isCompleted.set(false);

        const { container } = render(WebSocketStatus);

        const statusElement = container.querySelector('.status-disconnected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-disconnected class when there is an error', () => {
        connectionError.set('Connection error');

        const { container } = render(WebSocketStatus);

        const statusElement = container.querySelector('.status-disconnected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should render indicator element', () => {
        const { container } = render(WebSocketStatus);

        const indicator = container.querySelector('.indicator');
        expect(indicator).toBeInTheDocument();
    });

    it('should update status text when store values change', () => {
        const { rerender } = render(WebSocketStatus);

        expect(screen.getByText('Disconnected')).toBeInTheDocument();

        isConnecting.set(true);
        rerender({});

        expect(screen.getByText('Connecting...')).toBeInTheDocument();

        isConnecting.set(false);
        isConnected.set(true);
        rerender({});

        expect(screen.getByText('Connected')).toBeInTheDocument();
    });
});
