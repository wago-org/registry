import assert from "node:assert/strict";
import { highlightWat, supportsWat } from "../assets/js/wat.js";

for (const alias of ["wat", "wast", "wasm", "webassembly"]) {
    assert.equal(supportsWat(alias), true, `${alias} should select the WAT highlighter`);
}

const highlighted = highlightWat(`(module
  (; outer (; nested ;) comment ;)
  (func $add (param $left i32) (result i32)
    local.get $left
    i32.const 0x2a
    i32.add) ;; add the lanes
  (data "<unsafe>"))
`);

for (const expected of [
    '<span class="token keyword">module</span>',
    '<span class="token comment">(; outer (; nested ;) comment ;)</span>',
    '<span class="token variable">$add</span>',
    '<span class="token builtin">i32</span>',
    '<span class="token function">local.get</span>',
    '<span class="token number">0x2a</span>',
    '<span class="token string">"&lt;unsafe&gt;"</span>',
]) {
    assert.ok(highlighted.includes(expected), `missing highlighted output: ${expected}`);
}
assert.ok(!highlighted.includes("<unsafe>"), "code must remain HTML-escaped");

console.log("WAT syntax highlighting checks passed");
