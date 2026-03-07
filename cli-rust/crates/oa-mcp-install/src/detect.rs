//! Project type detection from repository contents.
//!
//! Detects the build system by checking for known manifest files in the
//! project root directory.

use std::path::Path;

use crate::types::ProjectType;

/// Detection entry: filename → project type, ordered by priority.
const DETECTION_TABLE: &[(&str, ProjectType)] = &[
    ("Cargo.toml", ProjectType::Rust),
    ("go.mod", ProjectType::Go),
    ("package.json", ProjectType::JavaScript),
    ("pyproject.toml", ProjectType::Python),
    ("setup.py", ProjectType::Python),
];

/// Detect the project type from the given directory.
///
/// Checks for known manifest files in priority order. Returns [`ProjectType::Unknown`]
/// if no known manifest is found.
pub fn detect_project_type(project_dir: &Path) -> ProjectType {
    for &(filename, project_type) in DETECTION_TABLE {
        if project_dir.join(filename).exists() {
            tracing::debug!(
                file = filename,
                project_type = %project_type,
                "detected project type"
            );
            return project_type;
        }
    }
    tracing::warn!(dir = %project_dir.display(), "could not detect project type");
    ProjectType::Unknown
}

/// Check if the project type is currently supported for building.
pub fn is_buildable(project_type: ProjectType) -> bool {
    matches!(project_type, ProjectType::Rust | ProjectType::Go)
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_detect_rust() {
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("Cargo.toml"), "[package]\nname = \"test\"").unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::Rust);
    }

    #[test]
    fn test_detect_go() {
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("go.mod"), "module example.com/test").unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::Go);
    }

    #[test]
    fn test_detect_javascript() {
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("package.json"), "{}").unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::JavaScript);
    }

    #[test]
    fn test_detect_python() {
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("pyproject.toml"), "[project]").unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::Python);
    }

    #[test]
    fn test_detect_unknown() {
        let dir = TempDir::new().unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::Unknown);
    }

    #[test]
    fn test_priority_rust_over_js() {
        // If both Cargo.toml and package.json exist, Rust wins (higher priority).
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("Cargo.toml"), "[package]").unwrap();
        fs::write(dir.path().join("package.json"), "{}").unwrap();
        assert_eq!(detect_project_type(dir.path()), ProjectType::Rust);
    }

    #[test]
    fn test_is_buildable() {
        assert!(is_buildable(ProjectType::Rust));
        assert!(is_buildable(ProjectType::Go));
        assert!(!is_buildable(ProjectType::JavaScript));
        assert!(!is_buildable(ProjectType::Python));
        assert!(!is_buildable(ProjectType::Unknown));
    }
}
