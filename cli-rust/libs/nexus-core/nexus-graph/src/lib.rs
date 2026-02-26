//! nexus-graph — 高性能图算法，通过 C ABI 暴露给 Go。
//!
//! 提供 PageRank / Label Propagation / 最短路径三个核心算法。

use libc::{c_float, c_int, size_t};
use petgraph::graph::UnGraph;
use petgraph::visit::EdgeRef;

// ---------------------------------------------------------------------------
// 1) PageRank — 节点重要性排序
// ---------------------------------------------------------------------------

/// 计算无向图的 PageRank。
///
/// - `edges`: 边列表，每两个 u32 为一条边 (src, dst)
/// - `n_edges`: 边数量
/// - `n_nodes`: 节点数量（节点 ID 从 0 开始）
/// - `damping`: 阻尼因子（通常 0.85）
/// - `iterations`: 迭代次数
/// - `out`: 长度为 `n_nodes` 的输出缓冲区
///
/// 返回 0 表示成功，-1 表示参数错误。
#[no_mangle]
pub unsafe extern "C" fn nexus_page_rank(
    edges: *const u32,
    n_edges: size_t,
    n_nodes: size_t,
    damping: c_float,
    iterations: size_t,
    out: *mut c_float,
) -> c_int {
    if edges.is_null() || out.is_null() || n_nodes == 0 {
        return -1;
    }
    let edge_data = unsafe { std::slice::from_raw_parts(edges, n_edges * 2) };
    let out_buf = unsafe { std::slice::from_raw_parts_mut(out, n_nodes) };

    page_rank_impl(edge_data, n_nodes, damping, iterations, out_buf);
    0
}

/// PageRank 内部实现。
fn page_rank_impl(
    edge_data: &[u32],
    n_nodes: usize,
    damping: f32,
    iterations: usize,
    out: &mut [f32],
) {
    let mut graph = UnGraph::<(), ()>::new_undirected();
    for _ in 0..n_nodes {
        graph.add_node(());
    }
    for chunk in edge_data.chunks_exact(2) {
        let (a, b) = (chunk[0] as u32, chunk[1] as u32);
        if (a as usize) < n_nodes && (b as usize) < n_nodes {
            graph.add_edge(a.into(), b.into(), ());
        }
    }

    // 初始化等概率
    let init = 1.0 / n_nodes as f32;
    let mut rank = vec![init; n_nodes];
    let mut next = vec![0.0f32; n_nodes];

    for _ in 0..iterations {
        next.fill((1.0 - damping) / n_nodes as f32);
        for node_idx in graph.node_indices() {
            let degree = graph.edges(node_idx).count();
            if degree == 0 {
                continue;
            }
            let share = rank[node_idx.index()] * damping / degree as f32;
            for edge in graph.edges(node_idx) {
                let target = edge.target().index();
                next[target] += share;
            }
        }
        std::mem::swap(&mut rank, &mut next);
    }

    out[..n_nodes].copy_from_slice(&rank);
}

// ---------------------------------------------------------------------------
// 2) Label Propagation — 社区检测
// ---------------------------------------------------------------------------

/// Label Propagation 社区检测。
///
/// - `edges`: 边列表，每两个 u32 为一条边
/// - `n_edges`: 边数量
/// - `n_nodes`: 节点数量
/// - `max_iter`: 最大迭代次数
/// - `out`: 长度为 `n_nodes` 的输出缓冲区（社区 ID）
///
/// 返回 0 成功，-1 参数错误。
#[no_mangle]
pub unsafe extern "C" fn nexus_label_propagation(
    edges: *const u32,
    n_edges: size_t,
    n_nodes: size_t,
    max_iter: size_t,
    out: *mut u32,
) -> c_int {
    if edges.is_null() || out.is_null() || n_nodes == 0 {
        return -1;
    }
    let edge_data = unsafe { std::slice::from_raw_parts(edges, n_edges * 2) };
    let out_buf = unsafe { std::slice::from_raw_parts_mut(out, n_nodes) };

    label_propagation_impl(edge_data, n_nodes, max_iter, out_buf);
    0
}

/// Label Propagation 内部实现。
fn label_propagation_impl(
    edge_data: &[u32],
    n_nodes: usize,
    max_iter: usize,
    out: &mut [u32],
) {
    // 构建邻接表
    let mut adj: Vec<Vec<usize>> = vec![vec![]; n_nodes];
    for chunk in edge_data.chunks_exact(2) {
        let (a, b) = (chunk[0] as usize, chunk[1] as usize);
        if a < n_nodes && b < n_nodes {
            adj[a].push(b);
            adj[b].push(a);
        }
    }

    // 初始化：每个节点自成一个社区
    let mut labels: Vec<u32> = (0..n_nodes as u32).collect();

    for _ in 0..max_iter {
        let mut changed = false;
        for node in 0..n_nodes {
            if adj[node].is_empty() {
                continue;
            }
            // 统计邻居标签频率，选最多的
            let mut freq = std::collections::HashMap::new();
            for &nb in &adj[node] {
                *freq.entry(labels[nb]).or_insert(0u32) += 1;
            }
            let best = freq.into_iter().max_by_key(|&(_, c)| c)
                .map(|(l, _)| l).unwrap_or(labels[node]);
            if best != labels[node] {
                labels[node] = best;
                changed = true;
            }
        }
        if !changed {
            break;
        }
    }

    out[..n_nodes].copy_from_slice(&labels);
}

// ---------------------------------------------------------------------------
// 3) Shortest Path — BFS 最短路径
// ---------------------------------------------------------------------------

/// BFS 最短路径。
///
/// - `edges`: 边列表
/// - `n_edges`: 边数量
/// - `n_nodes`: 节点数量
/// - `from`, `to`: 起止节点 ID
/// - `out_path`: 输出路径缓冲区（最大 `n_nodes` 长度）
/// - `out_len`: 写入实际路径长度
///
/// 返回 0 成功，-1 参数错误，-2 无路径。
#[no_mangle]
pub unsafe extern "C" fn nexus_shortest_path(
    edges: *const u32,
    n_edges: size_t,
    n_nodes: size_t,
    from: u32,
    to: u32,
    out_path: *mut u32,
    out_len: *mut size_t,
) -> c_int {
    if edges.is_null() || out_path.is_null() || out_len.is_null() || n_nodes == 0 {
        return -1;
    }
    if from as usize >= n_nodes || to as usize >= n_nodes {
        return -1;
    }
    let edge_data = unsafe { std::slice::from_raw_parts(edges, n_edges * 2) };
    let path_buf = unsafe { std::slice::from_raw_parts_mut(out_path, n_nodes) };

    match shortest_path_impl(edge_data, n_nodes, from as usize, to as usize) {
        Some(path) => {
            let len = path.len().min(n_nodes);
            for (i, &node) in path.iter().take(len).enumerate() {
                path_buf[i] = node as u32;
            }
            unsafe { *out_len = len };
            0
        }
        None => -2, // 无路径
    }
}

/// BFS 最短路径内部实现。
fn shortest_path_impl(
    edge_data: &[u32],
    n_nodes: usize,
    from: usize,
    to: usize,
) -> Option<Vec<usize>> {
    use std::collections::VecDeque;

    let mut adj: Vec<Vec<usize>> = vec![vec![]; n_nodes];
    for chunk in edge_data.chunks_exact(2) {
        let (a, b) = (chunk[0] as usize, chunk[1] as usize);
        if a < n_nodes && b < n_nodes {
            adj[a].push(b);
            adj[b].push(a);
        }
    }

    let mut visited = vec![false; n_nodes];
    let mut parent = vec![usize::MAX; n_nodes];
    let mut queue = VecDeque::new();

    visited[from] = true;
    queue.push_back(from);

    while let Some(node) = queue.pop_front() {
        if node == to {
            // 回溯路径
            let mut path = vec![to];
            let mut cur = to;
            while cur != from {
                cur = parent[cur];
                path.push(cur);
            }
            path.reverse();
            return Some(path);
        }
        for &nb in &adj[node] {
            if !visited[nb] {
                visited[nb] = true;
                parent[nb] = node;
                queue.push_back(nb);
            }
        }
    }
    None
}

// ---------------------------------------------------------------------------
// Rust 侧单元测试
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    // 测试图: 0--1--2--3, 0--2
    fn sample_edges() -> Vec<u32> {
        vec![0, 1, 1, 2, 2, 3, 0, 2]
    }

    #[test]
    fn test_page_rank_basic() {
        let edges = sample_edges();
        let mut out = vec![0.0f32; 4];
        let ret = unsafe {
            nexus_page_rank(edges.as_ptr(), 4, 4, 0.85, 20, out.as_mut_ptr())
        };
        assert_eq!(ret, 0);
        // 所有 rank 应为正且总和近似 1.0
        let sum: f32 = out.iter().sum();
        assert!((sum - 1.0).abs() < 0.01, "rank 总和={sum}");
        assert!(out.iter().all(|&r| r > 0.0));
    }

    #[test]
    fn test_label_propagation_basic() {
        // 两个分离子图: {0,1,2} 和 {3,4,5}
        let edges: Vec<u32> = vec![0, 1, 1, 2, 0, 2, 3, 4, 4, 5, 3, 5];
        let mut out = vec![0u32; 6];
        let ret = unsafe {
            nexus_label_propagation(edges.as_ptr(), 6, 6, 10, out.as_mut_ptr())
        };
        assert_eq!(ret, 0);
        // 同子图内标签应相同
        assert_eq!(out[0], out[1]);
        assert_eq!(out[1], out[2]);
        assert_eq!(out[3], out[4]);
        assert_eq!(out[4], out[5]);
        // 两组标签应不同
        assert_ne!(out[0], out[3]);
    }

    #[test]
    fn test_shortest_path_basic() {
        let edges = sample_edges(); // 0--1--2--3, 0--2
        let mut path = vec![0u32; 4];
        let mut len: usize = 0;
        let ret = unsafe {
            nexus_shortest_path(
                edges.as_ptr(), 4, 4, 0, 3,
                path.as_mut_ptr(), &mut len as *mut usize,
            )
        };
        assert_eq!(ret, 0);
        assert!(len >= 2, "路径长度={len}");
        assert_eq!(path[0], 0);
        assert_eq!(path[len - 1], 3);
    }

    #[test]
    fn test_shortest_path_no_path() {
        // 两个孤立节点
        let edges: Vec<u32> = vec![];
        let mut path = vec![0u32; 2];
        let mut len: usize = 0;
        let ret = unsafe {
            nexus_shortest_path(
                edges.as_ptr(), 0, 2, 0, 1,
                path.as_mut_ptr(), &mut len as *mut usize,
            )
        };
        assert_eq!(ret, -2, "无路径应返回 -2");
    }
}
