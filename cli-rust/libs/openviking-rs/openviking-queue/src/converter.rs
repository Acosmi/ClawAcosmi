// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Context → EmbeddingMsg conversion.
//!
//! Ported from `openviking/storage/queuefs/embedding_msg_converter.py`.

use openviking_core::Context;

use crate::embedding_msg::{EmbeddingContent, EmbeddingMsg};

/// Converter for [`Context`] objects to [`EmbeddingMsg`].
pub struct EmbeddingMsgConverter;

impl EmbeddingMsgConverter {
    /// Convert a `Context` into an `EmbeddingMsg`.
    ///
    /// Returns `None` if the context has no vectorization text.
    pub fn from_context(context: &Context) -> Option<EmbeddingMsg> {
        let text = context.vectorization_text();
        if text.is_empty() {
            return None;
        }

        let context_data = context.to_storage_value();
        Some(EmbeddingMsg::new(
            EmbeddingContent::Text(text.to_owned()),
            context_data,
        ))
    }
}
