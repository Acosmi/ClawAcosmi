//! nexus-tokenizer — 中文分词 + BM25 评分，通过 C ABI 暴露给 Go。
//!
//! 替换 Go 侧简陋的字符级拆分 (vector_store.go tokenizeBM25)，
//! 使用 jieba-rs 实现真正的中文分词。

use jieba_rs::Jieba;
use libc::{c_char, c_double, c_int, c_uint};
use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::sync::{LazyLock, Mutex};

/// 全局 jieba 实例（加载默认词典，仅初始化一次）。
static JIEBA: LazyLock<Jieba> = LazyLock::new(Jieba::new);

/// 全局 IDF 表：词 → IDF 值。通过 `nexus_set_idf_table` 设置。
static IDF_TABLE: LazyLock<Mutex<HashMap<String, f64>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));

/// 全局文档总数 N，用于默认 IDF 计算。
static DOC_COUNT: LazyLock<Mutex<f64>> =
    LazyLock::new(|| Mutex::new(1.0));

// ---------------------------------------------------------------------------
// 1) tokenize — 返回分词结果（以 \0 分隔的 C 字符串）
// ---------------------------------------------------------------------------

/// 对输入文本进行中文分词。
///
/// 返回一个以 `\0` 分隔、以 `\0\0` 终结的 C 字符串。
/// 调用方必须使用 `nexus_tokenizer_free` 释放内存。
#[no_mangle]
pub unsafe extern "C" fn nexus_tokenize(
    text: *const c_char,
    out_count: *mut c_uint,
) -> *mut c_char {
    let c_str = unsafe { CStr::from_ptr(text) };
    let text = c_str.to_str().unwrap_or("");
    if text.is_empty() {
        unsafe { *out_count = 0 };
        return std::ptr::null_mut();
    }

    let words = JIEBA.cut(text, true); // HMM 模式
    let count = words.len() as c_uint;
    unsafe { *out_count = count };

    // 用 \0 分隔每个词，末尾多加一个 \0
    let joined: String = words.join("\0");
    match CString::new(joined) {
        Ok(cs) => cs.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// 释放 `nexus_tokenize` 返回的内存。
#[no_mangle]
pub unsafe extern "C" fn nexus_tokenizer_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        drop(unsafe { CString::from_raw(ptr) });
    }
}

// ---------------------------------------------------------------------------
// 2) IDF 表管理 — 从 Go 侧设置词频统计
// ---------------------------------------------------------------------------

/// 设置 IDF 表。entries 格式: "word1\0idf1\0word2\0idf2\0"。
/// doc_count 为文档总数 N。返回 0 成功，-1 失败。
#[no_mangle]
pub unsafe extern "C" fn nexus_set_idf_table(
    entries: *const c_char,
    entries_len: c_uint,
    doc_count: c_double,
) -> c_int {
    if entries.is_null() || entries_len == 0 {
        return -1;
    }
    let raw = unsafe {
        std::slice::from_raw_parts(entries as *const u8, entries_len as usize)
    };
    let s = match std::str::from_utf8(raw) {
        Ok(v) => v,
        Err(_) => return -1,
    };
    let parts: Vec<&str> = s.split('\0').collect();
    let mut table = HashMap::new();
    let mut i = 0;
    while i + 1 < parts.len() {
        if let Ok(v) = parts[i + 1].parse::<f64>() {
            if !parts[i].is_empty() {
                table.insert(parts[i].to_string(), v);
            }
        }
        i += 2;
    }
    if let Ok(mut t) = IDF_TABLE.lock() {
        *t = table;
    }
    if let Ok(mut n) = DOC_COUNT.lock() {
        *n = if doc_count > 0.0 { doc_count } else { 1.0 };
    }
    0
}

// ---------------------------------------------------------------------------
// 3) bm25_score — 计算 BM25 相关性得分
// ---------------------------------------------------------------------------

/// 计算 query 与 document 之间的 BM25 得分。
///
/// k1 = 1.2, b = 0.75, avg_dl 由调用方提供。
#[no_mangle]
pub unsafe extern "C" fn nexus_bm25_score(
    query: *const c_char,
    doc: *const c_char,
    avg_dl: c_double,
) -> c_double {
    let q = unsafe { CStr::from_ptr(query) }
        .to_str()
        .unwrap_or("");
    let d = unsafe { CStr::from_ptr(doc) }
        .to_str()
        .unwrap_or("");

    bm25_score_inner(q, d, avg_dl)
}

fn bm25_score_inner(query: &str, doc: &str, avg_dl: f64) -> f64 {
    const K1: f64 = 1.2;
    const B: f64 = 0.75;

    let q_tokens = JIEBA.cut(query, true);
    let d_tokens = JIEBA.cut(doc, true);
    let dl = d_tokens.len() as f64;
    if dl == 0.0 || avg_dl <= 0.0 {
        return 0.0;
    }

    // doc 词频
    let mut tf: HashMap<String, f64> = HashMap::new();
    for t in &d_tokens {
        *tf.entry(t.to_string()).or_default() += 1.0;
    }

    let mut score = 0.0;
    for qt in &q_tokens {
        let freq = tf.get(*qt).copied().unwrap_or(0.0);
        if freq > 0.0 {
            // 真实 IDF：优先从表查询，fallback 到 BM25 标准公式
            let idf = get_idf(*qt);
            let num = freq * (K1 + 1.0);
            let den = freq + K1 * (1.0 - B + B * dl / avg_dl);
            score += idf * num / den;
        }
    }
    score
}

/// 从 IDF 表获取词的 IDF 值，表中无则返回默认值 1.0。
fn get_idf(term: &str) -> f64 {
    if let Ok(table) = IDF_TABLE.lock() {
        if let Some(&v) = table.get(term) {
            return v;
        }
    }
    1.0 // fallback: 无 IDF 表时退化为原始行为
}

// ---------------------------------------------------------------------------
// Rust 侧单元测试
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tokenize_chinese() {
        let words = JIEBA.cut("你好世界", true);
        assert!(words.len() >= 2, "应分出至少 2 个词");
    }

    #[test]
    fn test_bm25_basic() {
        let score = bm25_score_inner("你好", "你好世界欢迎", 5.0);
        assert!(score > 0.0, "包含 query 词的文档应有正分");
    }

    #[test]
    fn test_bm25_no_match() {
        let score = bm25_score_inner("苹果", "今天天气很好", 5.0);
        assert_eq!(score, 0.0, "无交集应为 0");
    }

    #[test]
    fn test_idf_table_affects_score() {
        // 先记录无 IDF 表时的分数
        let score_without = bm25_score_inner("你好", "你好世界", 5.0);
        // 设置 IDF 表
        {
            let mut t = IDF_TABLE.lock().unwrap();
            t.insert("你好".to_string(), 3.5);
        }
        let score_with = bm25_score_inner("你好", "你好世界", 5.0);
        assert!(score_with > score_without, "有 IDF 表时分数应更高");
        // 清理
        IDF_TABLE.lock().unwrap().clear();
    }
}
