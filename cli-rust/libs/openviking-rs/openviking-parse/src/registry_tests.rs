// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for `registry` module.

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use async_trait::async_trait;

    use crate::registry::ParserRegistry;
    use crate::types::*;

    // -----------------------------------------------------------------------
    // Stub parser for testing
    // -----------------------------------------------------------------------

    struct StubParser {
        name: String,
        extensions: Vec<String>,
    }

    impl StubParser {
        fn new(name: &str, extensions: &[&str]) -> Self {
            Self {
                name: name.to_owned(),
                extensions: extensions.iter().map(|s| s.to_string()).collect(),
            }
        }
    }

    #[async_trait]
    impl Parser for StubParser {
        async fn parse(&self, source: &str) -> Result<ParseResult, BoxError> {
            let root = ResourceNode::root();
            Ok(create_parse_result(
                root,
                Some(source.to_owned()),
                None,
                Some(self.name.clone()),
                None,
                None,
                None,
                None,
            ))
        }

        fn supported_extensions(&self) -> Vec<String> {
            self.extensions.clone()
        }
    }

    // -----------------------------------------------------------------------
    // Tests
    // -----------------------------------------------------------------------

    #[test]
    fn test_registry_new_empty() {
        let reg = ParserRegistry::new();
        assert!(reg.is_empty());
        assert_eq!(reg.len(), 0);
    }

    #[test]
    fn test_registry_register() {
        let mut reg = ParserRegistry::new();
        let parser = Arc::new(StubParser::new("markdown", &[".md", ".markdown"]));
        reg.register("markdown", parser);

        assert_eq!(reg.len(), 1);
        assert!(reg.get_parser("markdown").is_some());
        assert!(reg.get_parser("nonexistent").is_none());
    }

    #[test]
    fn test_registry_get_parser_for_file() {
        let mut reg = ParserRegistry::new();
        reg.register(
            "markdown",
            Arc::new(StubParser::new("markdown", &[".md", ".markdown"])),
        );
        reg.register("text", Arc::new(StubParser::new("text", &[".txt"])));

        assert!(reg.get_parser_for_file("document.md").is_some());
        assert!(reg.get_parser_for_file("readme.MARKDOWN").is_some());
        assert!(reg.get_parser_for_file("data.txt").is_some());
        assert!(reg.get_parser_for_file("image.png").is_none());
    }

    #[test]
    fn test_registry_unregister() {
        let mut reg = ParserRegistry::new();
        reg.register(
            "markdown",
            Arc::new(StubParser::new("markdown", &[".md"])),
        );
        assert_eq!(reg.len(), 1);

        reg.unregister("markdown");
        assert_eq!(reg.len(), 0);
        assert!(reg.get_parser_for_file("document.md").is_none());
    }

    #[test]
    fn test_registry_list_parsers() {
        let mut reg = ParserRegistry::new();
        reg.register("text", Arc::new(StubParser::new("text", &[".txt"])));
        reg.register(
            "markdown",
            Arc::new(StubParser::new("markdown", &[".md"])),
        );

        let parsers = reg.list_parsers();
        assert_eq!(parsers.len(), 2);
        assert!(parsers.contains(&"text".to_owned()));
        assert!(parsers.contains(&"markdown".to_owned()));
    }

    #[test]
    fn test_registry_list_supported_extensions() {
        let mut reg = ParserRegistry::new();
        reg.register(
            "markdown",
            Arc::new(StubParser::new("markdown", &[".md", ".markdown"])),
        );

        let exts = reg.list_supported_extensions();
        assert_eq!(exts.len(), 2);
        assert!(exts.contains(&".md".to_owned()));
        assert!(exts.contains(&".markdown".to_owned()));
    }

    #[tokio::test]
    async fn test_registry_parse_with_fallback() {
        let mut reg = ParserRegistry::new();
        reg.register("text", Arc::new(StubParser::new("text", &[".txt"])));
        reg.register(
            "markdown",
            Arc::new(StubParser::new("markdown", &[".md"])),
        );

        // Known extension -> markdown parser
        let result = reg.parse("document.md").await.unwrap();
        assert_eq!(result.parser_name, Some("markdown".to_owned()));

        // Unknown extension -> fallback to text
        let result = reg.parse("unknown.xyz").await.unwrap();
        assert_eq!(result.parser_name, Some("text".to_owned()));
    }

    #[tokio::test]
    async fn test_registry_parse_no_fallback() {
        let reg = ParserRegistry::new();
        // No parsers registered -> error
        let result = reg.parse("document.md").await;
        assert!(result.is_err());
    }
}
