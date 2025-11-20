import { describe, it, expect } from 'vitest';
import { parseAnsi, formatTimestamp } from './ansi';

describe('ANSI Color Parser', () => {
    describe('parseAnsi', () => {
        it('should handle text without ANSI codes', () => {
            const text = 'Hello World';
            const result = parseAnsi(text);
            expect(result).toBe('Hello World');
        });

        it('should convert red ANSI code', () => {
            const text = '\x1b[31mRed Text\x1b[0m';
            const result = parseAnsi(text);
            expect(result).toContain('<span class="ansi-red">');
            expect(result).toContain('</span>');
            expect(result).toContain('Red Text');
        });

        it('should convert green ANSI code', () => {
            const text = '\x1b[32mGreen Text\x1b[0m';
            const result = parseAnsi(text);
            expect(result).toContain('<span class="ansi-green">');
            expect(result).toContain('Green Text');
        });

        it('should handle reset code', () => {
            const text = '\x1b[31mRed\x1b[0m Normal';
            const result = parseAnsi(text);
            expect(result).toContain('<span class="ansi-red">');
            expect(result).toContain('</span>');
        });

        it('should convert all standard colors', () => {
            const colors = [
                { code: 30, name: 'black' },
                { code: 31, name: 'red' },
                { code: 32, name: 'green' },
                { code: 33, name: 'yellow' },
                { code: 34, name: 'blue' },
                { code: 35, name: 'magenta' },
                { code: 36, name: 'cyan' },
                { code: 37, name: 'white' }
            ];

            colors.forEach(({ code, name }) => {
                const text = `\x1b[${code}mText\x1b[0m`;
                const result = parseAnsi(text);
                expect(result).toContain(`<span class="ansi-${name}">`);
            });
        });

        it('should convert bright colors', () => {
            const brightColors = [
                { code: 90, name: 'bright-black' },
                { code: 91, name: 'bright-red' },
                { code: 92, name: 'bright-green' },
                { code: 93, name: 'bright-yellow' },
                { code: 94, name: 'bright-blue' },
                { code: 95, name: 'bright-magenta' },
                { code: 96, name: 'bright-cyan' },
                { code: 97, name: 'bright-white' }
            ];

            brightColors.forEach(({ code, name }) => {
                const text = `\x1b[${code}mText\x1b[0m`;
                const result = parseAnsi(text);
                expect(result).toContain(`<span class="ansi-${name}">`);
            });
        });

        it('should escape HTML special characters', () => {
            const text = '<div>&copy;</div>';
            const result = parseAnsi(text);
            expect(result).toContain('&lt;');
            expect(result).toContain('&gt;');
            expect(result).toContain('&amp;');
            expect(result).not.toContain('<div>');
        });

        it('should handle multiple ANSI codes', () => {
            const text = '\x1b[31mRed\x1b[0m \x1b[32mGreen\x1b[0m \x1b[34mBlue\x1b[0m';
            const result = parseAnsi(text);
            expect(result).toContain('ansi-red');
            expect(result).toContain('ansi-green');
            expect(result).toContain('ansi-blue');
        });

        it('should handle unknown ANSI codes gracefully', () => {
            const text = '\x1b[99mUnknown\x1b[0m';
            const result = parseAnsi(text);
            // Unknown codes should be skipped, but reset should still be processed
            expect(result).toContain('Unknown');
        });

        it('should handle consecutive ANSI codes', () => {
            const text = '\x1b[31m\x1b[1mBold Red\x1b[0m';
            const result = parseAnsi(text);
            expect(result).toContain('Bold Red');
        });

        it('should preserve text structure', () => {
            const text = 'Line 1\nLine 2\nLine 3';
            const result = parseAnsi(text);
            expect(result).toContain('Line 1\nLine 2\nLine 3');
        });

        it('should handle empty strings', () => {
            const result = parseAnsi('');
            expect(result).toBe('');
        });

        it('should handle only reset code', () => {
            const result = parseAnsi('\x1b[0m');
            expect(result).toBe('</span>');
        });
    });
});

describe('Timestamp Formatter', () => {
    describe('formatTimestamp', () => {
        it('should format valid timestamp', () => {
            // 2025-01-01 00:00:00Z
            const timestamp = new Date('2025-01-01T00:00:00Z').getTime();
            const result = formatTimestamp(timestamp);
            expect(result).toBe('2025-01-01 00:00:00Z');
        });

        it('should handle different valid timestamps', () => {
            const testCases = [
                {
                    timestamp: new Date('2025-06-15T12:30:45Z').getTime(),
                    expected: '2025-06-15 12:30:45Z'
                },
                {
                    timestamp: new Date('2024-12-31T23:59:59Z').getTime(),
                    expected: '2024-12-31 23:59:59Z'
                },
                {
                    timestamp: new Date('2025-01-01T01:02:03Z').getTime(),
                    expected: '2025-01-01 01:02:03Z'
                }
            ];

            testCases.forEach(({ timestamp, expected }) => {
                const result = formatTimestamp(timestamp);
                expect(result).toBe(expected);
            });
        });

        it('should return empty string for null', () => {
            const result = formatTimestamp(null);
            expect(result).toBe('');
        });

        it('should return empty string for undefined', () => {
            const result = formatTimestamp(undefined);
            expect(result).toBe('');
        });

        it('should return empty string for zero', () => {
            const result = formatTimestamp(0);
            expect(result).toBe('');
        });

        it('should return empty string for negative numbers', () => {
            const result = formatTimestamp(-1000);
            expect(result).toBe('');
        });

        it('should handle large timestamps', () => {
            // Year 2050
            const timestamp = new Date('2050-12-31T23:59:59Z').getTime();
            const result = formatTimestamp(timestamp);
            expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}Z$/);
            expect(result).toBe('2050-12-31 23:59:59Z');
        });

        it('should include Z suffix', () => {
            const timestamp = new Date('2025-01-01T00:00:00Z').getTime();
            const result = formatTimestamp(timestamp);
            expect(result.endsWith('Z')).toBe(true);
        });

        it('should have correct format length', () => {
            const timestamp = new Date('2025-01-01T00:00:00Z').getTime();
            const result = formatTimestamp(timestamp);
            // YYYY-MM-DD HH:MM:SSZ = 20 characters
            expect(result).toHaveLength(20);
        });

        it('should match YYYY-MM-DD HH:MM:SSZ format', () => {
            const timestamp = new Date('2025-06-15T14:30:45Z').getTime();
            const result = formatTimestamp(timestamp);
            expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}Z$/);
        });

        it('should handle leap year dates', () => {
            const timestamp = new Date('2024-02-29T12:00:00Z').getTime(); // Leap year
            const result = formatTimestamp(timestamp);
            expect(result).toBe('2024-02-29 12:00:00Z');
        });

        it('should handle end of month dates', () => {
            const testCases = [
                new Date('2025-01-31T12:00:00Z'),
                new Date('2025-04-30T12:00:00Z'),
                new Date('2025-09-30T12:00:00Z'),
                new Date('2025-12-31T12:00:00Z')
            ];

            testCases.forEach((date) => {
                const result = formatTimestamp(date.getTime());
                expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}Z$/);
            });
        });
    });
});
