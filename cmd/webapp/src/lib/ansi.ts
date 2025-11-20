/**
 * Parse ANSI escape codes in text and convert to HTML with CSS classes
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

/**
 * Parse ANSI escape codes in text and convert to HTML with CSS classes
 * @param text - Text containing ANSI codes
 * @returns HTML string with ANSI codes converted to spans
 */
export function parseAnsi(text: string): string {
    // eslint-disable-next-line no-control-regex
    const ansiRegex = /\x1b\[(\d+)m/g;

    // Escape HTML special characters first
    let result = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

    // Replace ANSI codes with HTML spans
    result = result.replace(ansiRegex, (match, code) => {
        const codeNum = parseInt(code, 10);
        if (codeNum === 0) {
            return '</span>'; // Reset
        }
        const color = colorMap[codeNum];
        return color ? `<span class="ansi-${color}">` : '';
    });

    return result;
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
