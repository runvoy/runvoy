/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import WebSocketStatus from './WebSocketStatus.svelte';

describe('WebSocketStatus', () => {
    afterEach(() => {
        cleanup();
    });

    const defaultProps = {
        isConnecting: false,
        isConnected: false,
        connectionError: null,
        isCompleted: false
    };

    it('should display "Done" when execution is completed', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isCompleted: true
            }
        });

        expect(screen.getByText('Done')).toBeInTheDocument();
    });

    it('should display "Connecting" when connecting', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnecting: true
            }
        });

        expect(screen.getByText('Connecting')).toBeInTheDocument();
    });

    it('should display "Live" when connected', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnected: true
            }
        });

        expect(screen.getByText('Live')).toBeInTheDocument();
    });

    it('should display "Error" when error is set', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                connectionError: 'Connection failed'
            }
        });

        expect(screen.getByText('Error')).toBeInTheDocument();
    });

    it('should display "Offline" when not connected, not connecting, and no error', () => {
        render(WebSocketStatus, {
            props: defaultProps
        });

        expect(screen.getByText('Offline')).toBeInTheDocument();
    });

    it('should prioritize completed status over connecting status', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isCompleted: true,
                isConnecting: true
            }
        });

        expect(screen.getByText('Done')).toBeInTheDocument();
        expect(screen.queryByText('Connecting')).not.toBeInTheDocument();
    });

    it('should prioritize completed status over connected status', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isCompleted: true,
                isConnected: true
            }
        });

        expect(screen.getByText('Done')).toBeInTheDocument();
        expect(screen.queryByText('Live')).not.toBeInTheDocument();
    });

    it('should prioritize connecting status over connected status', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnecting: true,
                isConnected: true
            }
        });

        expect(screen.getByText('Connecting')).toBeInTheDocument();
        expect(screen.queryByText('Live')).not.toBeInTheDocument();
    });

    it('should prioritize connecting status over error status', () => {
        render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnecting: true,
                connectionError: 'Some error'
            }
        });

        expect(screen.getByText('Connecting')).toBeInTheDocument();
        expect(screen.queryByText('Error')).not.toBeInTheDocument();
    });

    it('should apply status-completed class when execution is completed', () => {
        const { container } = render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isCompleted: true
            }
        });

        const statusElement = container.querySelector('.status-completed');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-connecting class when connecting', () => {
        const { container } = render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnecting: true
            }
        });

        const statusElement = container.querySelector('.status-connecting');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-connected class when connected', () => {
        const { container } = render(WebSocketStatus, {
            props: {
                ...defaultProps,
                isConnected: true
            }
        });

        const statusElement = container.querySelector('.status-connected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-disconnected class when disconnected', () => {
        const { container } = render(WebSocketStatus, {
            props: defaultProps
        });

        const statusElement = container.querySelector('.status-disconnected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should apply status-disconnected class when there is an error', () => {
        const { container } = render(WebSocketStatus, {
            props: {
                ...defaultProps,
                connectionError: 'Connection error'
            }
        });

        const statusElement = container.querySelector('.status-disconnected');
        expect(statusElement).toBeInTheDocument();
    });

    it('should render indicator element', () => {
        const { container } = render(WebSocketStatus, {
            props: defaultProps
        });

        const indicator = container.querySelector('.indicator');
        expect(indicator).toBeInTheDocument();
    });

    it('should update status text when props change', () => {
        const { rerender } = render(WebSocketStatus, {
            props: defaultProps
        });

        expect(screen.getByText('Offline')).toBeInTheDocument();

        rerender({
            ...defaultProps,
            isConnecting: true
        });

        expect(screen.getByText('Connecting')).toBeInTheDocument();

        rerender({
            ...defaultProps,
            isConnecting: false,
            isConnected: true
        });

        expect(screen.getByText('Live')).toBeInTheDocument();
    });
});
