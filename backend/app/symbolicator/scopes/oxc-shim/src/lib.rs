use std::os::raw::c_char;
use std::panic::{catch_unwind, AssertUnwindSafe};

use oxc_allocator::Allocator;
use oxc_ast::ast::{ArrowFunctionExpression, Class, Function};
use oxc_ast_visit::{walk, Visit};
use oxc_parser::Parser;
use oxc_span::SourceType;
use oxc_syntax::scope::ScopeFlags;

struct ScopeCollector {
    out: Vec<u32>,
}

impl<'a> Visit<'a> for ScopeCollector {
    fn visit_function(&mut self, it: &Function<'a>, flags: ScopeFlags) {
        let name = it.id.as_ref().map_or(it.span.start, |id| id.span.start);
        self.out.extend_from_slice(&[it.span.start, it.span.end, name]);
        walk::walk_function(self, it, flags);
    }

    fn visit_arrow_function_expression(&mut self, it: &ArrowFunctionExpression<'a>) {
        self.out
            .extend_from_slice(&[it.span.start, it.span.end, it.span.start]);
        walk::walk_arrow_function_expression(self, it);
    }

    fn visit_class(&mut self, it: &Class<'a>) {
        let name = it.id.as_ref().map_or(it.span.start, |id| id.span.start);
        self.out.extend_from_slice(&[it.span.start, it.span.end, name]);
        walk::walk_class(self, it);
    }
}

fn collect_scopes(src: &str) -> Option<Vec<u32>> {
    for source_type in [SourceType::mjs(), SourceType::cjs()] {
        let allocator = Allocator::default();
        let ret = Parser::new(&allocator, src, source_type).parse();
        if ret.panicked || !ret.errors.is_empty() {
            continue;
        }
        let mut collector = ScopeCollector { out: Vec::new() };
        collector.visit_program(&ret.program);
        return Some(collector.out);
    }
    None
}

const OXC_OK: i32 = 0;
const OXC_ERR_BAD_ARGS: i32 = 1;
const OXC_ERR_INVALID_UTF8: i32 = 2;
const OXC_ERR_PARSE: i32 = 3;
const OXC_ERR_PANIC: i32 = 4;

#[no_mangle]
pub extern "C" fn oxc_parse_scopes(
    src: *const c_char,
    len: usize,
    out: *mut *mut u32,
    out_len: *mut usize,
) -> i32 {
    if src.is_null() || out.is_null() || out_len.is_null() {
        return OXC_ERR_BAD_ARGS;
    }
    let bytes = unsafe { std::slice::from_raw_parts(src as *const u8, len) };
    let text = match std::str::from_utf8(bytes) {
        Ok(t) => t,
        Err(_) => return OXC_ERR_INVALID_UTF8,
    };
    match catch_unwind(AssertUnwindSafe(|| collect_scopes(text))) {
        Ok(Some(scopes)) => {
            let mut boxed = scopes.into_boxed_slice();
            unsafe {
                *out = boxed.as_mut_ptr();
                *out_len = boxed.len();
            }
            std::mem::forget(boxed);
            OXC_OK
        }
        Ok(None) => OXC_ERR_PARSE,
        Err(_) => OXC_ERR_PANIC,
    }
}

#[no_mangle]
pub extern "C" fn oxc_free_scopes(ptr: *mut u32, len: usize) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        drop(Vec::from_raw_parts(ptr, len, len));
    }
}
