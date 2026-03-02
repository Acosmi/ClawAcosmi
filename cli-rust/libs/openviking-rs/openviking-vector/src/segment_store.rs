// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! [`SegmentVectorStore`] — process-in-memory vector store backed by Qdrant segment.
//!
//! Each "collection" maps to a dedicated [`Segment`] instance stored on disk.
//! Operations (upsert, search, delete) go directly through the segment API
//! without any network I/O.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};

use common::budget::ResourcePermit;
use common::counter::hardware_accumulator::HwMeasurementAcc;
use common::counter::hardware_counter::HardwareCounterCell;
use parking_lot::RwLock;
use segment::data_types::named_vectors::NamedVectors;
use segment::data_types::query_context::QueryContext;
use segment::data_types::vectors::{QueryVector, DEFAULT_VECTOR_NAME};
use segment::entry::entry_point::{NonAppendableSegmentEntry, SegmentEntry};
use segment::index::hnsw_index::num_rayon_threads;
use segment::segment::Segment;
use segment::segment_constructor::build_segment;
use segment::segment_constructor::segment_builder::SegmentBuilder;
use segment::types::{
    Distance, ExtendedPointId, Filter, HnswConfig, HnswGlobalConfig, Indexes, Payload,
    PayloadFieldSchema, PayloadSchemaType, PayloadStorageType, QuantizationConfig, ScoredPoint,
    SearchParams, SegmentConfig, VectorDataConfig, VectorStorageDatatype, VectorStorageType,
    WithPayload, WithVector,
};
use uuid::Uuid;

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/// A search result returned by [`SegmentVectorStore`].
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SearchHit {
    pub id: String,
    pub score: f32,
    pub payload: HashMap<String, serde_json::Value>,
}

/// A scroll result (no score, just id + payload).
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ScrollHit {
    pub id: String,
    pub payload: HashMap<String, serde_json::Value>,
}

/// Configuration for creating a collection.
#[derive(Debug, Clone)]
pub struct CollectionConfig {
    /// Dense vector dimension.
    pub dimension: usize,
    /// Distance metric (Cosine, Dot, Euclid, Manhattan).
    pub distance: Distance,
    /// Whether to also create a sparse vector storage.
    pub sparse_vectors: bool,
    /// HNSW index configuration. `None` = Plain (brute-force) index.
    pub hnsw: Option<HnswConfig>,
    /// Quantization configuration. `None` = no quantization (full Float32).
    pub quantization: Option<QuantizationConfig>,
    /// Vector storage type. Default: `InRamChunkedMmap`.
    pub storage_type: VectorStorageType,
    /// Vector storage datatype. `None` = Float32.
    pub datatype: Option<VectorStorageDatatype>,
}

impl Default for CollectionConfig {
    fn default() -> Self {
        Self {
            dimension: 1536,
            distance: Distance::Cosine,
            sparse_vectors: false,
            hnsw: None,
            quantization: None,
            storage_type: VectorStorageType::InRamChunkedMmap,
            datatype: None,
        }
    }
}

// ---------------------------------------------------------------------------
// Internal state
// ---------------------------------------------------------------------------

struct CollectionHandle {
    segment: Segment,
    config: CollectionConfig,
    /// Monotonically increasing operation number for this segment.
    op_counter: AtomicU64,
}

impl CollectionHandle {
    fn next_op(&self) -> u64 {
        self.op_counter.fetch_add(1, Ordering::Relaxed) + 1
    }
}

// ---------------------------------------------------------------------------
// SegmentVectorStore
// ---------------------------------------------------------------------------

/// In-process vector store backed by Qdrant's `segment` library.
///
/// Thread-safe via `RwLock` over the collections map.
/// Individual `Segment` instances handle their own internal synchronisation.
pub struct SegmentVectorStore {
    /// `collection_name` → `CollectionHandle`.
    collections: RwLock<HashMap<String, CollectionHandle>>,
    /// Root directory for all segment data.
    data_dir: PathBuf,
}

impl SegmentVectorStore {
    /// Create a new store rooted at `data_dir`.
    ///
    /// The directory is created if it does not exist.
    pub fn new(data_dir: impl AsRef<Path>) -> std::io::Result<Self> {
        let data_dir = data_dir.as_ref().to_path_buf();
        std::fs::create_dir_all(&data_dir)?;
        Ok(Self {
            collections: RwLock::new(HashMap::new()),
            data_dir,
        })
    }

    // --- Collection management -----------------------------------------------

    /// Create a new collection. Returns `true` if created, `false` if it already exists.
    pub fn create_collection(
        &self,
        name: &str,
        cfg: &CollectionConfig,
    ) -> Result<bool, Box<dyn std::error::Error + Send + Sync>> {
        let mut map = self.collections.write();
        if map.contains_key(name) {
            return Ok(false);
        }

        let col_dir = self.data_dir.join(name);
        std::fs::create_dir_all(&col_dir)?;

        // Build SegmentConfig with a single named dense vector ("dense").
        //
        // NOTE: build_segment() creates an *appendable* (mutable) segment.
        // Appendable segments only support Plain index — HNSW graphs are built
        // during segment *optimization* (converting to non-appendable format).
        // We always use Indexes::Plain here and store the HNSW config in
        // CollectionHandle for future optimization support.
        let mut vector_data = HashMap::new();
        vector_data.insert(
            DEFAULT_VECTOR_NAME.to_owned(),
            VectorDataConfig {
                size: cfg.dimension,
                distance: cfg.distance,
                storage_type: cfg.storage_type,
                index: Indexes::Plain {},
                quantization_config: cfg.quantization.clone(),
                multivector_config: None,
                datatype: cfg.datatype,
            },
        );

        let seg_config = SegmentConfig {
            vector_data,
            sparse_vector_data: Default::default(),
            payload_storage_type: PayloadStorageType::Mmap,
        };

        let segment = build_segment(&col_dir, &seg_config, true)?;

        map.insert(
            name.to_owned(),
            CollectionHandle {
                segment,
                config: cfg.clone(),
                op_counter: AtomicU64::new(0),
            },
        );

        let index_label = if cfg.hnsw.is_some() {
            "Plain (HNSW config stored for optimization)"
        } else {
            "Plain"
        };
        log::info!(
            "Created segment collection: {name} (dim={}, index={index_label})",
            cfg.dimension
        );
        Ok(true)
    }

    /// Check if a collection exists.
    pub fn collection_exists(&self, name: &str) -> bool {
        self.collections.read().contains_key(name)
    }

    /// List all collection names.
    pub fn list_collections(&self) -> Vec<String> {
        self.collections.read().keys().cloned().collect()
    }

    /// Drop a collection, removing it from memory (disk data retained).
    pub fn drop_collection(&self, name: &str) -> bool {
        self.collections.write().remove(name).is_some()
    }

    // --- Point operations ----------------------------------------------------

    /// Upsert a point with a dense vector and optional payload.
    ///
    /// `point_id` is a UUID string which is converted to `ExtendedPointId::Uuid`.
    pub fn upsert(
        &self,
        collection: &str,
        point_id: &str,
        dense_vector: &[f32],
        payload: Option<&Payload>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let mut map = self.collections.write();
        let handle = map
            .get_mut(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let op_num = handle.next_op();
        let id = parse_point_id(point_id)?;
        let hw = HardwareCounterCell::disposable();

        // Build named vectors with the default dense vector.
        let vectors = NamedVectors::from_ref(DEFAULT_VECTOR_NAME, dense_vector.into());

        handle.segment.upsert_point(op_num, id, vectors, &hw)?;

        // Set payload if provided.
        if let Some(p) = payload {
            handle.segment.set_payload(op_num, id, p, &None, &hw)?;
        }

        Ok(())
    }

    /// Search for nearest neighbours in a collection.
    ///
    /// Returns up to `limit` results sorted by descending score.
    /// `search_params` allows tuning HNSW `ef` and quantization behaviour.
    /// Pass `None` for default parameters.
    pub fn search(
        &self,
        collection: &str,
        query_vector: &[f32],
        filter: Option<&Filter>,
        limit: usize,
        search_params: Option<&SearchParams>,
    ) -> Result<Vec<SearchHit>, Box<dyn std::error::Error + Send + Sync>> {
        let map = self.collections.read();
        let handle = map
            .get(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let query = QueryVector::Nearest(query_vector.to_vec().into());
        let query_context = QueryContext::new(usize::MAX, HwMeasurementAcc::disposable());
        let segment_query_context = query_context.get_segment_query_context();

        let default_params = SearchParams::default();
        let params = search_params.unwrap_or(&default_params);

        let results = handle.segment.search_batch(
            DEFAULT_VECTOR_NAME,
            &[&query],
            &WithPayload::from(true),
            &WithVector::from(false),
            filter,
            limit,
            Some(params),
            &segment_query_context,
        )?;

        // search_batch returns Vec<Vec<ScoredPoint>>, we want the first batch.
        let scored_points = results.into_iter().next().unwrap_or_default();

        Ok(scored_points
            .into_iter()
            .map(|sp| scored_point_to_hit(sp))
            .collect())
    }

    /// Delete a point by ID. Returns `true` if the point was deleted.
    pub fn delete(
        &self,
        collection: &str,
        point_id: &str,
    ) -> Result<bool, Box<dyn std::error::Error + Send + Sync>> {
        let mut map = self.collections.write();
        let handle = map
            .get_mut(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let op_num = handle.next_op();
        let id = parse_point_id(point_id)?;
        let hw = HardwareCounterCell::disposable();

        let deleted = handle.segment.delete_point(op_num, id, &hw)?;
        Ok(deleted)
    }

    /// Flush a collection's segment to disk.
    pub fn flush(&self, collection: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let map = self.collections.read();
        let handle = map
            .get(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        handle.segment.flush(true)?;
        Ok(())
    }

    /// Create a payload field index for faster filtered search.
    pub fn create_field_index(
        &self,
        collection: &str,
        field_name: &str,
        field_type: PayloadSchemaType,
    ) -> Result<bool, Box<dyn std::error::Error + Send + Sync>> {
        let mut map = self.collections.write();
        let handle = map
            .get_mut(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let op_num = handle.next_op();
        let hw = HardwareCounterCell::disposable();
        let key = field_name
            .parse()
            .map_err(|_| format!("invalid payload key: {field_name}"))?;
        let schema = PayloadFieldSchema::FieldType(field_type);

        let created = handle
            .segment
            .create_field_index(op_num, &key, Some(&schema), &hw)?;
        Ok(created)
    }

    /// Get the number of indexed points in a collection.
    pub fn point_count(&self, collection: &str) -> Option<usize> {
        let map = self.collections.read();
        map.get(collection)
            .map(|h| h.segment.available_point_count())
    }

    /// Scroll all points in a collection (no vector similarity, just enumerate).
    ///
    /// Returns up to `limit` points with their payloads.
    pub fn scroll(
        &self,
        collection: &str,
        limit: usize,
    ) -> Result<Vec<ScrollHit>, Box<dyn std::error::Error + Send + Sync>> {
        let map = self.collections.read();
        let handle = map
            .get(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let is_stopped = AtomicBool::new(false);
        let hw = HardwareCounterCell::disposable();

        let point_ids = handle.segment.read_filtered(
            None,
            Some(limit),
            None,
            &is_stopped,
            &hw,
        );

        let mut results = Vec::with_capacity(point_ids.len());
        for pid in point_ids {
            let payload_raw = handle.segment.payload(pid, &hw)?;
            let payload: HashMap<String, serde_json::Value> = payload_raw
                .into_iter()
                .map(|(k, v)| (k.to_string(), v))
                .collect();
            let id = match pid {
                ExtendedPointId::NumId(n) => n.to_string(),
                ExtendedPointId::Uuid(u) => u.to_string(),
            };
            results.push(ScrollHit { id, payload });
        }

        Ok(results)
    }

    /// Scroll points matching a Qdrant filter (JSON-encoded).
    ///
    /// `filter_json` is a JSON string representing a Qdrant `Filter`.
    /// Returns up to `limit` matching points with their payloads.
    pub fn scroll_filtered(
        &self,
        collection: &str,
        filter_json: &str,
        limit: usize,
    ) -> Result<Vec<ScrollHit>, Box<dyn std::error::Error + Send + Sync>> {
        let map = self.collections.read();
        let handle = map
            .get(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let filter: Filter = serde_json::from_str(filter_json)
            .map_err(|e| format!("invalid filter JSON: {e}"))?;

        let is_stopped = AtomicBool::new(false);
        let hw = HardwareCounterCell::disposable();

        let point_ids = handle.segment.read_filtered(
            None,
            Some(limit),
            Some(&filter),
            &is_stopped,
            &hw,
        );

        let mut results = Vec::with_capacity(point_ids.len());
        for pid in point_ids {
            let payload_raw = handle.segment.payload(pid, &hw)?;
            let payload: HashMap<String, serde_json::Value> = payload_raw
                .into_iter()
                .map(|(k, v)| (k.to_string(), v))
                .collect();
            let id = match pid {
                ExtendedPointId::NumId(n) => n.to_string(),
                ExtendedPointId::Uuid(u) => u.to_string(),
            };
            results.push(ScrollHit { id, payload });
        }

        Ok(results)
    }

    // --- Segment optimization ------------------------------------------------

    /// Optimize a collection by converting its appendable (Plain) segment into
    /// a non-appendable segment with an HNSW index.
    ///
    /// Returns `Ok(true)` if HNSW was built, `Ok(false)` if skipped (no HNSW
    /// config, empty collection, or collection not found).
    ///
    /// The caller provides a `stopped` flag that can be used to cancel the
    /// operation cooperatively.
    pub fn optimize_collection(
        &self,
        name: &str,
        stopped: &AtomicBool,
    ) -> Result<bool, Box<dyn std::error::Error + Send + Sync>> {
        let mut map = self.collections.write();
        let handle = match map.get_mut(name) {
            Some(h) => h,
            None => return Ok(false),
        };

        // Only optimize if the collection was created with HNSW config.
        let hnsw_config = match &handle.config.hnsw {
            Some(cfg) => cfg.clone(),
            None => return Ok(false),
        };

        // Nothing to optimize if the segment is empty.
        if handle.segment.available_point_count() == 0 {
            return Ok(false);
        }

        let col_dir = self.data_dir.join(name);

        // Build target SegmentConfig with HNSW index (non-appendable).
        let mut vector_data = HashMap::new();
        vector_data.insert(
            DEFAULT_VECTOR_NAME.to_owned(),
            VectorDataConfig {
                size: handle.config.dimension,
                distance: handle.config.distance,
                storage_type: handle.config.storage_type,
                index: Indexes::Hnsw(hnsw_config),
                quantization_config: handle.config.quantization.clone(),
                multivector_config: None,
                datatype: handle.config.datatype,
            },
        );

        let target_config = SegmentConfig {
            vector_data,
            sparse_vector_data: Default::default(),
            payload_storage_type: PayloadStorageType::Mmap,
        };

        // Create a temporary directory for the builder inside the collection dir.
        let temp_dir = tempfile::tempdir_in(&col_dir)
            .map_err(|e| format!("create temp dir for optimization: {e}"))?;

        // SegmentBuilder: copy data from existing segment → build HNSW graph.
        let mut builder =
            SegmentBuilder::new(temp_dir.path(), &target_config, &HnswGlobalConfig::default())?;

        builder.update(&[&handle.segment], stopped)?;

        let num_threads = num_rayon_threads(0) as u32;
        let permit = ResourcePermit::dummy(num_threads);
        let hw = HardwareCounterCell::disposable();
        let progress_tracker = common::progress_tracker::ProgressTracker::new_for_test();

        let new_segment = builder.build(
            &col_dir,
            Uuid::new_v4(),
            permit,
            stopped,
            &mut rand::rng(),
            &hw,
            progress_tracker,
        )?;

        let point_count = new_segment.available_point_count();

        // Replace the old segment (old Segment drops automatically, cleaning disk files).
        handle.segment = new_segment;

        log::info!(
            "Optimized segment collection: {name} (points={point_count}, index=HNSW)"
        );

        Ok(true)
    }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn parse_point_id(s: &str) -> Result<ExtendedPointId, Box<dyn std::error::Error + Send + Sync>> {
    let uuid = Uuid::parse_str(s)?;
    Ok(ExtendedPointId::Uuid(uuid))
}

fn scored_point_to_hit(sp: ScoredPoint) -> SearchHit {
    let id = match sp.id {
        ExtendedPointId::NumId(n) => n.to_string(),
        ExtendedPointId::Uuid(u) => u.to_string(),
    };

    let payload = sp
        .payload
        .map(|p| {
            p.into_iter()
                .map(|(k, v)| (k.to_string(), v))
                .collect::<HashMap<String, serde_json::Value>>()
        })
        .unwrap_or_default();

    SearchHit {
        id,
        score: sp.score,
        payload,
    }
}
