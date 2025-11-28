/**
 * Parse ANSI escape codes in text and convert to structured color segments
 */

interface ColorMap {
    [key: number]: string;
}

const colorMap: ColorMap = {
    30: 'black',
    31: 'red',
    32: 'green',
    33: 'yellow',
    34: 'blue',
    35: 'magenta',
    36: 'cyan',
    37: 'white',
    90: 'bright-black',
    91: 'bright-red',
    92: 'bright-green',
    93: 'bright-yellow',
    94: 'bright-blue',
    95: 'bright-magenta',
    96: 'bright-cyan',
    97: 'bright-white'
};

export interface AnsiSegment {
    text: string;
    className?: string;
}

/**
 * Parse ANSI escape codes in text and convert to structured segments
 * @param text - Text containing ANSI codes
 * @returns Array of segments with optional ANSI class names
 */
export function parseAnsi(text: string): AnsiSegment[] {
    const escapeChar = String.fromCharCode(0x1b);
    const ansiRegex = new RegExp(`${escapeChar}\\[(\\d+)m`, 'g');

    const segments: AnsiSegment[] = [];
    let currentClass: string | undefined;
    let lastIndex = 0;

    for (const match of text.matchAll(ansiRegex)) {
        if (match.index === undefined) {
            continue;
        }

        if (match.index > lastIndex) {
            segments.push({ text: text.slice(lastIndex, match.index), className: currentClass });
        }

        const codeNum = parseInt(match[1], 10);
        if (codeNum === 0) {
            currentClass = undefined;
        } else {
            const color = colorMap[codeNum];
            currentClass = color ? `ansi-${color}` : currentClass;
        }

        lastIndex = match.index + match[0].length;
    }

    if (lastIndex < text.length) {
        segments.push({ text: text.slice(lastIndex), className: currentClass });
    }

    return segments;
}

/**
 * Format timestamp in milliseconds to YYYY-MM-DD HH:MM:SSZ format
 * @param timestamp - Unix timestamp in milliseconds
 * @returns Formatted timestamp string
 */
export function formatTimestamp(timestamp: number | null | undefined): string {
    if (!timestamp || timestamp <= 0) {
        return '';
    }
    const date = new Date(timestamp);
    // Format as YYYY-MM-DD HH:MM:SSZ
    return date.toISOString().replace('T', ' ').substring(0, 19) + 'Z';
}
