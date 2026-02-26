/// Error formatting utilities.
///
/// Provides functions for formatting error chains into human-readable
/// strings, including the full chain of source causes.
///
/// Source: `src/infra/errors.ts`

/// Format an error and its entire chain of causes into a readable string.
///
/// Walks the error source chain and produces a multi-line string
/// showing each cause indented under the previous one.
///
/// # Examples
///
/// ```
/// use std::io;
/// use oa_infra::errors::format_error_chain;
///
/// let err = io::Error::new(io::ErrorKind::NotFound, "file not found");
/// let formatted = format_error_chain(&err);
/// assert!(formatted.contains("file not found"));
/// ```
pub fn format_error_chain(err: &dyn std::error::Error) -> String {
    let mut parts = vec![err.to_string()];
    let mut source = err.source();
    while let Some(cause) = source {
        parts.push(format!("  caused by: {cause}"));
        source = cause.source();
    }
    parts.join("\n")
}

/// Format an error message from any error type.
///
/// Returns the display representation of the error.
pub fn format_error_message(err: &dyn std::error::Error) -> String {
    err.to_string()
}

/// Format an uncaught error with its full stack / chain.
///
/// For errors with a source chain, includes all causes.
/// Returns the full formatted chain suitable for logging or display.
pub fn format_uncaught_error(err: &dyn std::error::Error) -> String {
    format_error_chain(err)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_error_chain_single() {
        let err = std::io::Error::new(std::io::ErrorKind::NotFound, "file not found");
        let result = format_error_chain(&err);
        assert_eq!(result, "file not found");
    }

    #[test]
    fn test_format_error_chain_with_source() {
        #[derive(Debug)]
        struct InnerError;

        impl std::fmt::Display for InnerError {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(f, "inner error")
            }
        }

        impl std::error::Error for InnerError {}

        #[derive(Debug)]
        struct OuterError {
            source: InnerError,
        }

        impl std::fmt::Display for OuterError {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(f, "outer error")
            }
        }

        impl std::error::Error for OuterError {
            fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
                Some(&self.source)
            }
        }

        let err = OuterError {
            source: InnerError,
        };
        let result = format_error_chain(&err);
        assert_eq!(result, "outer error\n  caused by: inner error");
    }

    #[test]
    fn test_format_error_message() {
        let err = std::io::Error::new(std::io::ErrorKind::PermissionDenied, "access denied");
        assert_eq!(format_error_message(&err), "access denied");
    }

    #[test]
    fn test_format_uncaught_error() {
        let err = std::io::Error::new(std::io::ErrorKind::Other, "unexpected failure");
        let result = format_uncaught_error(&err);
        assert!(result.contains("unexpected failure"));
    }
}
