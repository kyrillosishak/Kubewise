//! Build script for generating protobuf code
//!
//! This build script generates Rust code from protobuf definitions.
//! If protoc is not available, it will skip code generation.

use std::path::PathBuf;
use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Re-run if proto files change
    println!("cargo:rerun-if-changed=../../../proto/predictor/v1/predictor.proto");

    // Check if protoc is available
    let protoc_available =
        std::env::var("PROTOC").is_ok() || Command::new("protoc").arg("--version").output().is_ok();

    if !protoc_available {
        println!("cargo:warning=protoc not found, skipping proto generation");
        println!("cargo:warning=Install protoc or set PROTOC env var to generate proto code");
        return Ok(());
    }

    // Output directory for generated code
    let out_dir = PathBuf::from(std::env::var("OUT_DIR")?);

    // Compile protobuf definitions
    tonic_build::configure()
        .build_server(false) // Agent only needs client
        .build_client(true)
        .out_dir(&out_dir)
        .compile(
            &["../../../proto/predictor/v1/predictor.proto"],
            &["../../../proto"],
        )?;

    Ok(())
}
