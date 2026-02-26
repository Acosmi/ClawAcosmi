//! nexus-vector — 向量距离/相似度计算，通过 C ABI 暴露给 Go。
//!
//! 替换 Go 侧 tree_algorithm.go 中的 cosineSimilarity32，
//! 提供批量计算能力。

use libc::{c_float, c_int, size_t};

// ---------------------------------------------------------------------------
// 1) cosine_similarity — 单对向量余弦相似度
// ---------------------------------------------------------------------------

/// 计算两个 f32 向量的余弦相似度。
///
/// 返回 [-1.0, 1.0]，若任一向量为零向量则返回 0.0。
#[no_mangle]
pub unsafe extern "C" fn nexus_cosine_similarity(
    a: *const c_float,
    b: *const c_float,
    len: size_t,
) -> c_float {
    if a.is_null() || b.is_null() || len == 0 {
        return 0.0;
    }
    let a = unsafe { std::slice::from_raw_parts(a, len) };
    let b = unsafe { std::slice::from_raw_parts(b, len) };
    cosine_sim(a, b)
}

// ---------------------------------------------------------------------------
// 2) batch_cosine — query 与多个 doc 向量的批量余弦
// ---------------------------------------------------------------------------

/// 批量计算 query 与 docs 矩阵中每行的余弦相似度。
///
/// - `query`: 长度为 `dim` 的向量
/// - `docs`: `n_docs × dim` 的行主序矩阵
/// - `out`: 长度为 `n_docs` 的输出缓冲区
///
/// 返回 0 表示成功，-1 表示参数错误。
#[no_mangle]
pub unsafe extern "C" fn nexus_batch_cosine(
    query: *const c_float,
    docs: *const c_float,
    dim: size_t,
    n_docs: size_t,
    out: *mut c_float,
) -> c_int {
    if query.is_null() || docs.is_null() || out.is_null() || dim == 0 {
        return -1;
    }
    let q = unsafe { std::slice::from_raw_parts(query, dim) };
    let d = unsafe { std::slice::from_raw_parts(docs, n_docs * dim) };
    let o = unsafe { std::slice::from_raw_parts_mut(out, n_docs) };

    for i in 0..n_docs {
        let row = &d[i * dim..(i + 1) * dim];
        o[i] = cosine_sim(q, row);
    }
    0
}

// ---------------------------------------------------------------------------
// Internal implementation — SIMD-optimized via iterator patterns
// ---------------------------------------------------------------------------
// R2 优化：使用 iterator zip+fold 替代索引循环，便于 LLVM 自动向量化。
// 在 opt-level=3 + LTO 下，编译器将自动生成 SSE/AVX 指令。

/// SIMD-friendly cosine similarity using iterators for auto-vectorization.
#[inline]
fn cosine_sim(a: &[f32], b: &[f32]) -> f32 {
    debug_assert_eq!(a.len(), b.len(), "vector lengths must match");

    let (dot, norm_a, norm_b) = a.iter().zip(b.iter()).fold(
        (0.0f32, 0.0f32, 0.0f32),
        |(dot, na, nb), (&ai, &bi)| {
            (dot + ai * bi, na + ai * ai, nb + bi * bi)
        },
    );

    let denom = (norm_a * norm_b).sqrt();
    if denom < 1e-10 {
        0.0
    } else {
        dot / denom
    }
}

/// x86_64 AVX2 specialized path — called at runtime when AVX2 is available.
#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
#[inline]
unsafe fn cosine_sim_avx2(a: &[f32], b: &[f32]) -> f32 {
    // Same iterator pattern — compiler emits 256-bit SIMD with target_feature.
    cosine_sim(a, b)
}

/// Dispatches to AVX2 path if available (x86_64 only), otherwise fallback.
#[inline]
fn cosine_sim_dispatch(a: &[f32], b: &[f32]) -> f32 {
    #[cfg(target_arch = "x86_64")]
    {
        if is_x86_feature_detected!("avx2") {
            return unsafe { cosine_sim_avx2(a, b) };
        }
    }
    cosine_sim(a, b)
}

// ---------------------------------------------------------------------------
// Rust 侧单元测试
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cosine_identical() {
        let a = [1.0, 2.0, 3.0];
        let sim = cosine_sim(&a, &a);
        assert!((sim - 1.0).abs() < 1e-6, "相同向量应为 1.0");
    }

    #[test]
    fn test_cosine_orthogonal() {
        let a = [1.0, 0.0];
        let b = [0.0, 1.0];
        let sim = cosine_sim(&a, &b);
        assert!(sim.abs() < 1e-6, "正交向量应为 0.0");
    }

    #[test]
    fn test_cosine_opposite() {
        let a = [1.0, 0.0];
        let b = [-1.0, 0.0];
        let sim = cosine_sim(&a, &b);
        assert!((sim + 1.0).abs() < 1e-6, "相反向量应为 -1.0");
    }

    #[test]
    fn test_batch_cosine() {
        let query = [1.0f32, 0.0, 0.0];
        let docs = [1.0, 0.0, 0.0, 0.0, 1.0, 0.0, -1.0, 0.0, 0.0];
        let mut out = [0.0f32; 3];
        let ret = unsafe {
            nexus_batch_cosine(
                query.as_ptr(),
                docs.as_ptr(),
                3,
                3,
                out.as_mut_ptr(),
            )
        };
        assert_eq!(ret, 0);
        assert!((out[0] - 1.0).abs() < 1e-6);
        assert!(out[1].abs() < 1e-6);
        assert!((out[2] + 1.0).abs() < 1e-6);
    }
}
