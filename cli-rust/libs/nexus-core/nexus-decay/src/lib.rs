//! nexus-decay — 记忆衰减算法，通过 C ABI 暴露给 Go。
//!
//! 替换 Go 侧 memory_decay.go 中的 ComputeEffectiveImportance，
//! 提供批量衰减计算能力。

use libc::{c_double, c_int, size_t};

/// 衰减常量 — 与 Go 侧 memory_decay.go 保持一致。
const HALF_LIFE_DAYS: f64 = 30.0;
const MIN_DECAY: f64 = 0.01;
const LN2: f64 = 0.693_147_180_559_945;

// ---------------------------------------------------------------------------
// 1) decay — 单条记忆衰减计算
// ---------------------------------------------------------------------------

/// 计算单条记忆的有效重要性。
///
/// 公式：effective = base × decay_factor × exp(-ln2 × days / 30) × (1 + ln(1+access) × 0.1)
/// 结果 clamp 到 [MIN_DECAY, 1.0]。
#[no_mangle]
pub extern "C" fn nexus_decay(
    base_importance: c_double,
    decay_factor: c_double,
    days_since_access: c_double,
    access_count: c_int,
) -> c_double {
    decay_inner(base_importance, decay_factor, days_since_access, access_count as f64)
}

// ---------------------------------------------------------------------------
// 2) batch_decay — 批量衰减
// ---------------------------------------------------------------------------

/// 批量记忆衰减参数（C 侧结构体）。
#[repr(C)]
pub struct DecayInput {
    pub base_importance: c_double,
    pub decay_factor: c_double,
    pub days_since_access: c_double,
    pub access_count: c_int,
}

/// 批量计算衰减。
///
/// - `inputs`: 长度为 `count` 的 DecayInput 数组
/// - `out`: 长度为 `count` 的输出缓冲区
///
/// 返回 0 成功，-1 参数错误。
#[no_mangle]
pub unsafe extern "C" fn nexus_batch_decay(
    inputs: *const DecayInput,
    count: size_t,
    out: *mut c_double,
) -> c_int {
    if inputs.is_null() || out.is_null() || count == 0 {
        return -1;
    }
    let inp = unsafe { std::slice::from_raw_parts(inputs, count) };
    let o = unsafe { std::slice::from_raw_parts_mut(out, count) };

    for i in 0..count {
        o[i] = decay_inner(
            inp[i].base_importance,
            inp[i].decay_factor,
            inp[i].days_since_access,
            inp[i].access_count as f64,
        );
    }
    0
}

// ---------------------------------------------------------------------------
// 内部实现
// ---------------------------------------------------------------------------

#[inline]
fn decay_inner(base: f64, decay_factor: f64, days: f64, access_count: f64) -> f64 {
    let days = if days < 0.0 { 0.0 } else { days };
    let time_decay = (-LN2 * days / HALF_LIFE_DAYS).exp();
    let recency_boost = 1.0 + (1.0 + access_count).ln() * 0.1;
    let effective = base * decay_factor * time_decay * recency_boost;
    effective.clamp(MIN_DECAY, 1.0)
}

// ---------------------------------------------------------------------------
// Rust 侧单元测试
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_decay_no_time() {
        // 刚访问过，不衰减
        let v = decay_inner(0.8, 1.0, 0.0, 0.0);
        assert!((v - 0.8).abs() < 0.01);
    }

    #[test]
    fn test_decay_half_life() {
        // 30 天后衰减为约一半
        let v = decay_inner(1.0, 1.0, 30.0, 0.0);
        assert!((v - 0.5).abs() < 0.05, "30天后应约为 0.5, got {v}");
    }

    #[test]
    fn test_decay_floor() {
        // 极长时间后不低于 MIN_DECAY
        let v = decay_inner(0.1, 0.1, 365.0, 0.0);
        assert!(v >= MIN_DECAY);
    }

    #[test]
    fn test_batch_decay() {
        let inputs = [
            DecayInput { base_importance: 0.8, decay_factor: 1.0, days_since_access: 0.0, access_count: 0 },
            DecayInput { base_importance: 1.0, decay_factor: 1.0, days_since_access: 30.0, access_count: 0 },
        ];
        let mut out = [0.0f64; 2];
        let ret = unsafe { nexus_batch_decay(inputs.as_ptr(), 2, out.as_mut_ptr()) };
        assert_eq!(ret, 0);
        assert!((out[0] - 0.8).abs() < 0.01);
        assert!((out[1] - 0.5).abs() < 0.05);
    }
}
