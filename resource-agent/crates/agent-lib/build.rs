//! Build script for generating protobuf code
//!
//! This build script generates Rust code from protobuf definitions.
//! If protoc is not available or proto files are missing, it will skip code generation.
//! The proto types are already defined manually in src/proto/mod.rs as a fallback.

use std::path::Path;
use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Re-run if proto files change
    println!("cargo:rerun-if-changed=../../../proto/predictor/v1/predictor.proto");

    // Check if proto file exists
    let proto_path = Path::new("../../../proto/predictor/v1/predictor.proto");
    if !proto_path.exists() {
        println!("cargo:warning=Proto file not found, using pre-defined types in src/proto/mod.rs");
        return Ok(());
    }

    // Check if protoc is available
    let protoc_available =
        std::env::var("PROTOC").is_ok() || Command::new("protoc").arg("--version").output().is_ok();

    if !protoc_available {
        println!("cargo:warning=protoc not found, using pre-defined types in src/proto/mod.rs");
        return Ok(());
    }

    // Proto generation is optional - types are already defined in src/proto/mod.rs
    // Uncomment below to regenerate from .proto files
    /*
    let out_dir = PathBuf::from(std::env::var("OUT_DIR")?);
    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .out_dir(&out_dir)
        .compile(
            &["../../../proto/predictor/v1/predictor.proto"],
            &["../../../proto"],
        )?;
    */

    Ok(())
}
