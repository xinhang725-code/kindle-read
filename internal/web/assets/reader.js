/* SPDX-License-Identifier: AGPL-3.0-only */
(function (root, factory) {
    if (typeof module === 'object' && module.exports) { module.exports = factory(); }
    else { root.Reader = factory(); }
}(this, function () {
    'use strict';
    function boundaries(text) {
        var out = [0], i = 0, c;
        while (i < text.length) {
            c = text.charCodeAt(i++);
            if (c >= 0xD800 && c <= 0xDBFF && i < text.length && text.charCodeAt(i) >= 0xDC00 && text.charCodeAt(i) <= 0xDFFF) { i++; }
            out.push(i);
        }
        return out;
    }
    // fits(text, parts) measures the exact rendered page in the host DOM.
    function paginate(paragraphs, fits) {
        if (typeof fits !== 'function') { throw new Error('measurement required'); }
        var pages = [], current = [], i, offset, points, lo, hi, mid, best, candidate, text;
        function finish() { if (current.length) { pages.push({parts: current, anchor: {paragraph: current[0].paragraph, offset: current[0].offset}}); current = []; } }
        function trial(part) { var list = current.concat([part]); return fits(list.map(function (p) { return p.text; }).join('\n'), list); }
        for (i = 0; i < paragraphs.length; i++) {
            offset = 0; text = paragraphs[i];
            while (offset < text.length) {
                points = boundaries(text.slice(offset)); lo = 1; hi = points.length - 1; best = 0;
                while (lo <= hi) {
                    mid = Math.floor((lo + hi) / 2);
                    candidate = {paragraph: i, offset: offset, text: text.slice(offset, offset + points[mid])};
                    if (trial(candidate)) { best = mid; lo = mid + 1; } else { hi = mid - 1; }
                }
                if (!best) { if (current.length) { finish(); continue; } throw new Error('viewport too small'); }
                current.push({paragraph: i, offset: offset, text: text.slice(offset, offset + points[best])}); offset += points[best];
                if (offset < text.length) { finish(); }
            }
        }
        finish(); return pages;
    }
    function locate(pages, anchor) {
        var found = 0, i, a;
        if (!anchor) { return 0; }
        for (i = 0; i < pages.length; i++) {
            a = pages[i].anchor;
            if (a.paragraph < anchor.paragraph || (a.paragraph === anchor.paragraph && a.offset <= anchor.offset)) { found = i; } else { break; }
        }
        return found;
    }
    return {paginate: paginate, locate: locate};
}));
