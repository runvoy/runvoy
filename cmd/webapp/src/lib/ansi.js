/**
 * Parse ANSI escape codes in text and convert to HTML with CSS classes
 * @param {string} text - Text containing ANSI codes
 * @returns {string} HTML string with ANSI codes converted to spans
 */
export function parseAnsi(text) {
    // eslint-disable-next-line no-control-regex
    const ansiRegex = /\x1b\[(\d+)m/g;

    // Escape HTML special characters first
    let result = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

    // Color code mappings
    const colorMap = {
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

    // Replace ANSI codes with HTML spans
    result = result.replace(ansiRegex, (match, code) => {
        if (code === '0') {
            return '</span>'; // Reset
        }
        const color = colorMap[code];
        return color ? `<span class="ansi-${color}">` : '';
    });

    return result;
}

/**
 * Format timestamp in milliseconds to YYYY-MM-DD HH:MM:SSZ format
 * @param {number} timestamp - Unix timestamp in milliseconds
 * @returns {string} Formatted timestamp string
 */
export function formatTimestamp(timestamp) {
    if (!timestamp || timestamp <= 0) {
        return '';
    }
    const date = new Date(timestamp);
    // Format as YYYY-MM-DD HH:MM:SSZ
    return date.toISOString().replace('T', ' ').substring(0, 19) + 'Z';
}
