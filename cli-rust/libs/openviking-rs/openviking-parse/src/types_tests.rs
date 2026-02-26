// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for `types` module.

#[cfg(test)]
mod tests {
    use crate::types::*;
    use std::collections::HashMap;

    // -----------------------------------------------------------------------
    // Enum tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_node_type_serde() {
        let root = NodeType::Root;
        let json = serde_json::to_string(&root).unwrap();
        assert_eq!(json, "\"root\"");

        let section = NodeType::Section;
        let json = serde_json::to_string(&section).unwrap();
        assert_eq!(json, "\"section\"");

        let parsed: NodeType = serde_json::from_str("\"root\"").unwrap();
        assert_eq!(parsed, NodeType::Root);
    }

    #[test]
    fn test_resource_category_serde() {
        let doc = ResourceCategory::Document;
        let json = serde_json::to_string(&doc).unwrap();
        assert_eq!(json, "\"document\"");

        let media = ResourceCategory::Media;
        let json = serde_json::to_string(&media).unwrap();
        assert_eq!(json, "\"media\"");
    }

    #[test]
    fn test_document_type_serde() {
        let pdf = DocumentType::Pdf;
        let json = serde_json::to_string(&pdf).unwrap();
        assert_eq!(json, "\"pdf\"");

        let md = DocumentType::Markdown;
        let json = serde_json::to_string(&md).unwrap();
        assert_eq!(json, "\"markdown\"");
    }

    // -----------------------------------------------------------------------
    // MediaStrategy tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_calculate_media_strategy_text_only() {
        assert_eq!(calculate_media_strategy(0, 100), MediaStrategy::TextOnly);
    }

    #[test]
    fn test_calculate_media_strategy_extract() {
        // 2 images in 100 lines → ratio 0.02 < 0.3 → extract
        assert_eq!(calculate_media_strategy(2, 100), MediaStrategy::Extract);
    }

    #[test]
    fn test_calculate_media_strategy_full_page_vlm_ratio() {
        // 40 images in 100 lines → ratio 0.4 > 0.3 → full_page_vlm
        assert_eq!(
            calculate_media_strategy(40, 100),
            MediaStrategy::FullPageVlm
        );
    }

    #[test]
    fn test_calculate_media_strategy_full_page_vlm_count() {
        // 5 images → >= 5 threshold → full_page_vlm
        assert_eq!(calculate_media_strategy(5, 100), MediaStrategy::FullPageVlm);
    }

    #[test]
    fn test_calculate_media_strategy_zero_lines() {
        // 3 images, 0 lines → extract (can't compute ratio)
        assert_eq!(calculate_media_strategy(3, 0), MediaStrategy::Extract);
    }

    // -----------------------------------------------------------------------
    // format_table_to_markdown tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_format_table_empty() {
        assert_eq!(format_table_to_markdown(&[], true), "");
    }

    #[test]
    fn test_format_table_with_header() {
        let rows = vec![
            vec!["Name".into(), "Age".into()],
            vec!["Alice".into(), "30".into()],
            vec!["Bob".into(), "25".into()],
        ];
        let table = format_table_to_markdown(&rows, true);
        assert!(table.contains("| Name"));
        assert!(table.contains("| ----"));
        assert!(table.contains("| Alice"));
        assert!(table.contains("| Bob"));
    }

    #[test]
    fn test_format_table_without_header() {
        let rows = vec![vec!["A".into(), "B".into()], vec!["C".into(), "D".into()]];
        let table = format_table_to_markdown(&rows, false);
        // No separator line.
        assert!(!table.contains("---"));
    }

    // -----------------------------------------------------------------------
    // ResourceNode tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_resource_node_root() {
        let root = ResourceNode::root();
        assert_eq!(root.node_type, NodeType::Root);
        assert_eq!(root.level, 0);
        assert!(root.children.is_empty());
        assert_eq!(root.content_type, "text");
    }

    #[test]
    fn test_resource_node_section() {
        let sec = ResourceNode::section("Introduction", 1);
        assert_eq!(sec.node_type, NodeType::Section);
        assert_eq!(sec.title, Some("Introduction".to_owned()));
        assert_eq!(sec.level, 1);
    }

    #[test]
    fn test_resource_node_add_child() {
        let mut root = ResourceNode::root();
        root.add_child(ResourceNode::section("Chapter 1", 1));
        root.add_child(ResourceNode::section("Chapter 2", 1));
        assert_eq!(root.children.len(), 2);
        assert_eq!(root.descendant_count(), 2);
    }

    #[test]
    fn test_resource_node_get_abstract() {
        // With meta["abstract"]
        let mut node = ResourceNode::root();
        node.meta.insert(
            "abstract".to_owned(),
            serde_json::Value::String("Existing abstract".to_owned()),
        );
        assert_eq!(node.get_abstract(256), "Existing abstract");

        // Fallback to title
        let sec = ResourceNode::section("My Title", 1);
        assert_eq!(sec.get_abstract(256), "My Title");

        // Truncation
        let sec2 = ResourceNode::section("A very long title that exceeds limit", 1);
        let abs = sec2.get_abstract(10);
        assert!(abs.len() <= 10);
        assert!(abs.ends_with("..."));
    }

    #[test]
    fn test_resource_node_serde_roundtrip() {
        let mut root = ResourceNode::root();
        root.title = Some("Test Document".to_owned());
        root.add_child(ResourceNode::section("Section 1", 1));
        root.add_child(ResourceNode::section("Section 2", 1));

        let json = serde_json::to_string(&root).unwrap();
        let parsed: ResourceNode = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.title, Some("Test Document".to_owned()));
        assert_eq!(parsed.children.len(), 2);
        assert_eq!(parsed.children[0].title, Some("Section 1".to_owned()));
    }

    #[test]
    fn test_resource_node_is_text_file() {
        let mut node = ResourceNode::root();
        node.content_path = Some("document.md".to_owned());
        assert!(node.is_text_file());

        node.content_path = Some("image.png".to_owned());
        assert!(!node.is_text_file());

        node.content_path = None;
        assert!(!node.is_text_file());
    }

    // -----------------------------------------------------------------------
    // ParseResult tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_parse_result_success() {
        let root = ResourceNode::root();
        let result = create_parse_result(root, None, None, None, None, None, None, None);
        assert!(result.success());
    }

    #[test]
    fn test_parse_result_with_warnings() {
        let root = ResourceNode::root();
        let result = create_parse_result(
            root,
            None,
            None,
            None,
            None,
            None,
            None,
            Some(vec!["warning 1".into()]),
        );
        assert!(!result.success());
    }

    #[test]
    fn test_parse_result_get_all_nodes() {
        let mut root = ResourceNode::root();
        let mut ch1 = ResourceNode::section("Ch1", 1);
        ch1.add_child(ResourceNode::section("Ch1.1", 2));
        root.add_child(ch1);
        root.add_child(ResourceNode::section("Ch2", 1));

        let result = create_parse_result(root, None, None, None, None, None, None, None);
        let all = result.get_all_nodes();
        // root + Ch1 + Ch1.1 + Ch2 = 4
        assert_eq!(all.len(), 4);
    }

    #[test]
    fn test_parse_result_get_sections() {
        let mut root = ResourceNode::root();
        root.add_child(ResourceNode::section("L1", 1));
        root.add_child(ResourceNode::section("L2", 2));
        root.add_child(ResourceNode::section("L3", 3));

        let result = create_parse_result(root, None, None, None, None, None, None, None);
        let sections = result.get_sections(1, 2);
        assert_eq!(sections.len(), 2);
    }

    #[test]
    fn test_parse_result_serde_roundtrip() {
        let mut root = ResourceNode::root();
        root.add_child(ResourceNode::section("Test", 1));

        let result = create_parse_result(
            root,
            Some("/tmp/test.md".into()),
            Some("markdown".into()),
            Some("MarkdownParser".into()),
            Some("2.0".into()),
            Some(0.5),
            Some(HashMap::new()),
            None,
        );

        let json = serde_json::to_string(&result).unwrap();
        let parsed: ParseResult = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.source_path, Some("/tmp/test.md".into()));
        assert_eq!(parsed.parser_name, Some("MarkdownParser".into()));
        assert!(parsed.success());
        assert_eq!(parsed.root.children.len(), 1);
    }

    #[test]
    fn test_create_parse_result_defaults() {
        let root = ResourceNode::root();
        let result = create_parse_result(root, None, None, None, None, None, None, None);
        assert_eq!(result.parser_version, Some("2.0".to_owned()));
        assert!(result.parse_timestamp.is_none());
    }

    #[test]
    fn test_create_parse_result_with_time() {
        let root = ResourceNode::root();
        let result = create_parse_result(root, None, None, None, None, Some(1.5), None, None);
        assert!(result.parse_timestamp.is_some());
        assert_eq!(result.parse_time, Some(1.5));
    }

    // -----------------------------------------------------------------------
    // CJK 字符边界安全测试 (DEF-OVK-P3-C1-1)
    // -----------------------------------------------------------------------

    #[test]
    fn test_get_abstract_cjk_no_panic() {
        // 中文字符每个占 3 字节，max_length=5 会落在第二个字符中间
        let mut node = ResourceNode::root();
        node.meta.insert(
            "content".to_owned(),
            serde_json::Value::String("你好世界こんにちは세계".to_owned()),
        );
        // 不应 panic，应安全截断到字符边界
        let abs = node.get_abstract(5);
        assert!(!abs.is_empty());
        // 5 字节只能容纳 1 个中文字符 (3 字节)，其余被截断
        assert!(abs.len() <= 8); // 3 bytes + "..." = 6, but truncation logic may vary
    }

    #[test]
    fn test_get_abstract_cjk_exact_boundary() {
        let mut node = ResourceNode::root();
        // "你好" = 6 bytes, max_length=6 恰好在边界上
        node.meta.insert(
            "content".to_owned(),
            serde_json::Value::String("你好世".to_owned()),
        );
        let abs = node.get_abstract(6);
        assert!(!abs.is_empty());
    }

    #[test]
    fn test_get_overview_cjk_no_panic() {
        let mut node = ResourceNode::root();
        // 2000 个中文字符 = 6000 字节，超过 1000 字节会触发截断
        node.meta.insert(
            "content".to_owned(),
            serde_json::Value::String("中".repeat(2000)),
        );
        let ov = node.get_overview(100);
        assert!(!ov.is_empty());
        assert!(ov.ends_with("..."));
    }

    #[test]
    fn test_get_abstract_mixed_scripts() {
        // 混合 ASCII + CJK + 韩语，测试各种字符边界
        let mut node = ResourceNode::root();
        node.meta.insert(
            "content".to_owned(),
            serde_json::Value::String("Hello你好세계World".to_owned()),
        );
        for max_len in 1..30 {
            // 任意长度都不应 panic
            let _ = node.get_abstract(max_len);
        }
    }
}
