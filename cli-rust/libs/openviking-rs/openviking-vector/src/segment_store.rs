// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! [`SegmentVectorStore`] — process-in-memory vector store backed by Qdrant segment.
//!
//! Each "collection" maps to a dedicated [`Segment`] instance stored on disk.
//! Operations (upsert, search, delete) go directly through the segment API
//! without any network I/O.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};

use common::counter::hardware_accumulator::HwMeasurementAcc;
use common::counter::hardware_counter::HardwareCounterCell;
use parking_lot::RwLock;
use segment::data_types::named_vectors::NamedVectors;
use segment::data_types::query_context::QueryContext;
use segment::data_types::vectors::{QueryVector, DEFAULT_VECTOR_NAME};
use segment::entry::entry_point::{NonAppendableSegmentEntry, SegmentEntry};
use segment::segment::Segment;
use segment::segment_constructor::build_segment;
use segment::types::{
    Distance, ExtendedPointId, Filter, Indexes, Payload, PayloadFieldSchema, PayloadSchemaType,
    PayloadStorageType, ScoredPoint, SearchParams, SegmentConfig, VectorDataConfig,
    VectorStorageType, WithPayload, WithVector,
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

/// Configuration for creating a collection.
#[derive(Debug, Clone)]
pub struct CollectionConfig {
    /// Dense vector dimension.
    pub dimension: usize,
    /// Distance metric (Cosine, Dot, Euclid).
    pub distance: Distance,
    /// Whether to also create a sparse vector storage.
    pub sparse_vectors: bool,
}

impl Default for CollectionConfig {
    fn default() -> Self {
        Self {
            dimension: 1536,
            distance: Distance::Cosine,
            sparse_vectors: false,
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
        let mut vector_data = HashMap::new();
        vector_data.insert(
            DEFAULT_VECTOR_NAME.to_owned(),
            VectorDataConfig {
                size: cfg.dimension,
                distance: cfg.distance,
                storage_type: VectorStorageType::InRamChunkedMmap,
                index: Indexes::Plain {},
                quantization_config: None,
                multivector_config: None,
                datatype: None,
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

        log::info!("Created segment collection: {name} (dim={})", cfg.dimension);
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
    pub fn search(
        &self,
        collection: &str,
        query_vector: &[f32],
        filter: Option<&Filter>,
        limit: usize,
    ) -> Result<Vec<SearchHit>, Box<dyn std::error::Error + Send + Sync>> {
        let map = self.collections.read();
        let handle = map
            .get(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;

        let query = QueryVector::Nearest(query_vector.to_vec().into());
        let query_context = QueryContext::new(usize::MAX, HwMeasurementAcc::disposable());
        let segment_query_context = query_context.get_segment_query_context();

        let results = handle.segment.search_batch(
            DEFAULT_VECTOR_NAME,
            &[&query],
            &WithPayload::from(true),
            &WithVector::from(false),
            filter,
            limit,
            Some(&SearchParams::default()),
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
