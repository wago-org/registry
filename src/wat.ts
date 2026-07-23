// Lightweight WebAssembly Text highlighter. WAT is S-expression-shaped, but a
// Lisp grammar misses the parts readers care about: instructions, value types,
// nested block comments, identifiers, and escaped data strings.

const WAT_ALIASES = new Set(["wat", "wast", "wasm", "webassembly"]);

const KEYWORDS = new Set([
    "module", "component", "type", "sub", "rec", "func", "param", "result", "local",
    "global", "memory", "table", "tag", "import", "export", "start", "data", "elem",
    "declare", "item", "offset", "mut", "shared",
    "block", "loop", "if", "then", "else", "end", "try", "catch", "catch_all",
    "delegate", "throw", "rethrow", "return", "call", "call_indirect", "return_call",
    "return_call_indirect", "br", "br_if", "br_table", "unreachable", "nop", "drop",
    "select",
]);

const BUILTINS = new Set([
    "i32", "i64", "f32", "f64", "v128",
    "i8", "i16", "f16",
    "funcref", "externref", "anyref", "eqref", "i31ref", "structref", "arrayref",
    "exnref", "nullref", "nullfuncref", "nullexternref",
    "ref", "extern", "any", "eq", "i31", "struct", "array", "none", "noextern",
]);

function escapeHtml(value: string): string {
    return value.replace(/[&<>]/g, (ch) => ch === "&" ? "&amp;" : ch === "<" ? "&lt;" : "&gt;");
}

function token(kind: string, value: string): string {
    return `<span class="token ${kind}">${escapeHtml(value)}</span>`;
}

function isDelimiter(ch: string): boolean {
    return ch === "" || /\s/.test(ch) || ch === "(" || ch === ")" || ch === '"' || ch === ";";
}

function isNumber(word: string): boolean {
    return /^[+-]?(?:(?:0x[\da-f](?:_?[\da-f])*(?:\.[\da-f](?:_?[\da-f])*)?(?:p[+-]?\d(?:_?\d)*)?)|(?:\d(?:_?\d)*(?:\.\d(?:_?\d)*)?(?:e[+-]?\d(?:_?\d)*)?)|inf|nan(?::0x[\da-f](?:_?[\da-f])*)?)$/i.test(word);
}

function classifyWord(word: string): string | null {
    const lower = word.toLowerCase();
    if (KEYWORDS.has(lower)) return "keyword";
    if (BUILTINS.has(lower)) return "builtin";
    if (isNumber(lower)) return "number";
    if (lower === "true" || lower === "false" || lower === "null") return "constant";
    if (/^(?:offset|align)=/i.test(word)) return "property";
    // WAT's namespaced opcodes carry most of the useful semantic signal:
    // local.get, i32.add, v128.load32_lane, memory.copy, ref.cast, …
    if (/^[a-z][\w-]*(?:\.[\w-]+)+$/i.test(word)) return "function";
    if (word.startsWith("@")) return "property";
    return null;
}

export function supportsWat(language: string): boolean {
    return WAT_ALIASES.has(language.toLowerCase());
}

export function highlightWat(source: string): string {
    let out = "";
    let i = 0;

    while (i < source.length) {
        const ch = source[i];

        if (/\s/.test(ch)) {
            const start = i++;
            while (i < source.length && /\s/.test(source[i])) i++;
            out += escapeHtml(source.slice(start, i));
            continue;
        }

        if (source.startsWith(";;", i)) {
            const start = i;
            i += 2;
            while (i < source.length && source[i] !== "\n") i++;
            out += token("comment", source.slice(start, i));
            continue;
        }

        if (source.startsWith("(;", i)) {
            const start = i;
            let depth = 1;
            i += 2;
            while (i < source.length && depth > 0) {
                if (source.startsWith("(;", i)) {
                    depth++;
                    i += 2;
                } else if (source.startsWith(";)", i)) {
                    depth--;
                    i += 2;
                } else {
                    i++;
                }
            }
            out += token("comment", source.slice(start, i));
            continue;
        }

        if (ch === '"') {
            const start = i++;
            while (i < source.length) {
                if (source[i] === "\\") {
                    i += Math.min(2, source.length - i);
                } else if (source[i++] === '"') {
                    break;
                }
            }
            out += token("string", source.slice(start, i));
            continue;
        }

        if (ch === "(" || ch === ")") {
            out += token("punctuation", ch);
            i++;
            continue;
        }

        const start = i++;
        while (i < source.length && !isDelimiter(source[i])) i++;
        const word = source.slice(start, i);
        if (word.startsWith("$")) {
            out += token("variable", word);
            continue;
        }
        const kind = classifyWord(word);
        out += kind ? token(kind, word) : escapeHtml(word);
    }

    return out;
}
