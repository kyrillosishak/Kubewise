//! CLI integration tests

use std::process::Command;

/// Test that the CLI shows help
#[test]
fn test_cli_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "CLI help should succeed");
    assert!(
        stdout.contains("Container Resource Predictor"),
        "Should show app name"
    );
    assert!(stdout.contains("get"), "Should show get command");
    assert!(stdout.contains("apply"), "Should show apply command");
    assert!(stdout.contains("approve"), "Should show approve command");
    assert!(stdout.contains("costs"), "Should show costs command");
    assert!(stdout.contains("debug"), "Should show debug command");
}

/// Test that the CLI shows version
#[test]
fn test_cli_version() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "--version"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "CLI version should succeed");
    assert!(stdout.contains("crp"), "Should show binary name");
}

/// Test get recommendations subcommand help
#[test]
fn test_get_recommendations_help() {
    let output = Command::new("cargo")
        .args([
            "run",
            "-p",
            "crp-cli",
            "--",
            "get",
            "recommendations",
            "--help",
        ])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(
        output.status.success(),
        "Get recommendations help should succeed"
    );
    assert!(
        stdout.contains("--namespace"),
        "Should show namespace option"
    );
    assert!(
        stdout.contains("--deployment"),
        "Should show deployment option"
    );
}

/// Test get models subcommand help
#[test]
fn test_get_models_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "get", "models", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Get models help should succeed");
    assert!(
        stdout.contains("--active-only"),
        "Should show active-only option"
    );
}

/// Test apply command help
#[test]
fn test_apply_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "apply", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Apply help should succeed");
    assert!(stdout.contains("--dry-run"), "Should show dry-run option");
}

/// Test approve command help
#[test]
fn test_approve_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "approve", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Approve help should succeed");
    assert!(stdout.contains("--approver"), "Should show approver option");
    assert!(stdout.contains("--reason"), "Should show reason option");
}

/// Test costs show subcommand help
#[test]
fn test_costs_show_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "costs", "show", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Costs show help should succeed");
    assert!(
        stdout.contains("--namespace"),
        "Should show namespace option"
    );
}

/// Test costs savings subcommand help
#[test]
fn test_costs_savings_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "costs", "savings", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Costs savings help should succeed");
    assert!(stdout.contains("--since"), "Should show since option");
}

/// Test debug predictions subcommand help
#[test]
fn test_debug_predictions_help() {
    let output = Command::new("cargo")
        .args([
            "run",
            "-p",
            "crp-cli",
            "--",
            "debug",
            "predictions",
            "--help",
        ])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(
        output.status.success(),
        "Debug predictions help should succeed"
    );
    assert!(
        stdout.contains("deployment"),
        "Should show deployment argument"
    );
}

/// Test debug agent subcommand help
#[test]
fn test_debug_agent_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "debug", "agent", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Debug agent help should succeed");
    assert!(stdout.contains("node"), "Should show node argument");
}

/// Test debug export subcommand help
#[test]
fn test_debug_export_help() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "debug", "export", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(output.status.success(), "Debug export help should succeed");
    assert!(stdout.contains("--since"), "Should show since option");
    assert!(stdout.contains("--output"), "Should show output option");
    assert!(
        stdout.contains("--namespace"),
        "Should show namespace option"
    );
}

/// Test format option
#[test]
fn test_format_option() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(stdout.contains("--format"), "Should show format option");
    assert!(stdout.contains("table"), "Should show table format");
    assert!(stdout.contains("json"), "Should show json format");
}

/// Test api-url option
#[test]
fn test_api_url_option() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "--help"])
        .output()
        .expect("Failed to execute command");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(stdout.contains("--api-url"), "Should show api-url option");
    assert!(stdout.contains("CRP_API_URL"), "Should show env var");
}

/// Test invalid command error handling
#[test]
fn test_invalid_command() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "invalid-command"])
        .output()
        .expect("Failed to execute command");

    assert!(!output.status.success(), "Invalid command should fail");

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("error") || stderr.contains("invalid"),
        "Should show error message"
    );
}

/// Test missing required argument error handling
#[test]
fn test_missing_argument() {
    let output = Command::new("cargo")
        .args(["run", "-p", "crp-cli", "--", "apply"])
        .output()
        .expect("Failed to execute command");

    assert!(!output.status.success(), "Missing argument should fail");

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("required") || stderr.contains("error"),
        "Should show error about missing argument"
    );
}
