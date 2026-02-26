/**
 * nexus_memfs.h — C ABI for the virtual filesystem memory store.
 *
 * Provides UHMS permanent memory management via a hierarchical VFS
 * with L0/L1/L2 tiered context loading (inspired by OpenViking).
 *
 * Must call nexus_memfs_init() once before any other function.
 */

#ifndef NEXUS_MEMFS_H
#define NEXUS_MEMFS_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/**
 * Initialize the global MemoryFS store.
 *
 * @param root_path  Directory for persisted filesystem data.
 * @return 0 on success, negative on error.
 */
int nexus_memfs_init(const char *root_path);

/**
 * Write a permanent memory entry.
 *
 * @param tenant_id   Tenant identifier.
 * @param user_id     User identifier.
 * @param memory_id   Unique memory UUID string.
 * @param category    "decisions" | "facts" | "emotions" | "todos".
 * @param content     L2 full content.
 * @param l0_abstract L0 one-sentence summary (~100 tokens).
 * @param l1_overview L1 structured overview (~2k tokens).
 * @return 0 on success, negative on error.
 */
int nexus_memfs_write_memory(const char *tenant_id, const char *user_id,
                             const char *memory_id, const char *category,
                             const char *content, const char *l0_abstract,
                             const char *l1_overview);

/**
 * Read content at a VFS path and tier level.
 *
 * @param tenant_id  Tenant identifier.
 * @param user_id    User identifier.
 * @param path       Relative path, e.g. "permanent/decisions/{id}.md".
 * @param level      0=L0(abstract), 1=L1(overview), 2=L2(detail).
 * @param out_buf    Output buffer.
 * @param buf_len    Size of out_buf.
 * @param out_len    Actual bytes written (excluding null terminator).
 * @return 0 on success, negative on error.
 */
int nexus_memfs_read(const char *tenant_id, const char *user_id,
                     const char *path, int level, char *out_buf, size_t buf_len,
                     size_t *out_len);

/**
 * List directory entries. Returns JSON array.
 *
 * @param path     Relative directory path. Empty string for root.
 * @param out_buf  Output buffer for JSON string.
 * @return 0 on success, negative on error.
 */
int nexus_memfs_list_dir(const char *tenant_id, const char *user_id,
                         const char *path, char *out_buf, size_t buf_len,
                         size_t *out_len);

/**
 * Search for memories matching keywords.
 *
 * Returns JSON array of SearchHit sorted by relevance.
 *
 * @param query        Search keywords (space-separated).
 * @param max_results  Maximum results to return.
 * @return 0 on success, negative on error.
 */
int nexus_memfs_search(const char *tenant_id, const char *user_id,
                       const char *query, int max_results, char *out_buf,
                       size_t buf_len, size_t *out_len);

/**
 * Delete a memory by ID.
 *
 * @return 0 on success, negative on error.
 */
int nexus_memfs_delete(const char *tenant_id, const char *user_id,
                       const char *memory_id);

/**
 * Evict a tenant+user filesystem from memory.
 * Persisted data is not affected.
 *
 * @return 0 on success, negative on error.
 */
int nexus_memfs_cleanup(const char *tenant_id, const char *user_id);

/**
 * Get total memory count for a tenant+user.
 *
 * @return count (>= 0) on success, negative on error.
 */
int nexus_memfs_memory_count(const char *tenant_id, const char *user_id);

/**
 * Create a session archive directory.
 *
 * @param index       Archive index (0, 1, 2, ...).
 * @param l0_summary  One-line summary for L0.
 * @param l1_overview Structured overview for L1.
 * @return 0 on success, negative on error.
 */
int nexus_memfs_create_archive_dir(const char *tenant_id, const char *user_id,
                                   int index, const char *l0_summary,
                                   const char *l1_overview);

/**
 * Get the next archive index for a tenant+user.
 *
 * @return index (>= 0) on success, negative on error.
 */
int nexus_memfs_next_archive_index(const char *tenant_id, const char *user_id);

/**
 * Search with full retrieval trace for visualization.
 *
 * Returns JSON of SearchTrace containing traversal steps, scores, and hits.
 *
 * @param query        Search keywords (space-separated).
 * @param max_results  Maximum results to return.
 * @return 0 on success, negative on error.
 */
int nexus_memfs_search_with_trace(const char *tenant_id, const char *user_id,
                                  const char *query, int max_results,
                                  char *out_buf, size_t buf_len,
                                  size_t *out_len);

#ifdef __cplusplus
}
#endif

#endif /* NEXUS_MEMFS_H */
