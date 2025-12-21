//! Recommendation-related CLI commands

use anyhow::Result;
use tabled::Tabled;

use crate::client::{ApiClient, ApplyRequest, ApproveRequest, ModelList, RecommendationList};
use crate::output::{
    color_confidence, color_status, format_bytes, format_cpu, print_success, print_warning,
    OutputFormat,
};

/// Row for recommendations table
#[derive(Tabled)]
struct RecommendationRow {
    #[tabled(rename = "ID")]
    id: String,
    #[tabled(rename = "Namespace")]
    namespace: String,
    #[tabled(rename = "Deployment")]
    deployment: String,
    #[tabled(rename = "CPU Req")]
    cpu_request: String,
    #[tabled(rename = "CPU Lim")]
    cpu_limit: String,
    #[tabled(rename = "Mem Req")]
    memory_request: String,
    #[tabled(rename = "Mem Lim")]
    memory_limit: String,
    #[tabled(rename = "Confidence")]
    confidence: String,
    #[tabled(rename = "Status")]
    status: String,
}

/// Row for models table
#[derive(Tabled)]
struct ModelRow {
    #[tabled(rename = "Version")]
    version: String,
    #[tabled(rename = "Created")]
    created_at: String,
    #[tabled(rename = "Accuracy")]
    accuracy: String,
    #[tabled(rename = "Size")]
    size: String,
    #[tabled(rename = "Active")]
    active: String,
    #[tabled(rename = "Rollbacks")]
    rollbacks: String,
}

/// Get recommendations with optional filters
pub async fn get_recommendations(
    client: &ApiClient,
    namespace: Option<String>,
    deployment: Option<String>,
    status: Option<String>,
    format: OutputFormat,
) -> Result<()> {
    let path = match &namespace {
        Some(ns) => format!("api/v1/recommendations/{}", ns),
        None => "api/v1/recommendations".to_string(),
    };

    let result: RecommendationList = client.get(&path).await?;

    // Filter by deployment and status if specified
    let filtered: Vec<_> = result
        .recommendations
        .into_iter()
        .filter(|r| {
            deployment
                .as_ref()
                .map(|d| r.deployment.contains(d))
                .unwrap_or(true)
        })
        .filter(|r| {
            status
                .as_ref()
                .map(|s| r.status.eq_ignore_ascii_case(s))
                .unwrap_or(true)
        })
        .collect();

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&filtered)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            if filtered.is_empty() {
                print_warning("No recommendations found");
                return Ok(());
            }

            let rows: Vec<RecommendationRow> = filtered
                .iter()
                .map(|r| RecommendationRow {
                    id: truncate_id(&r.id),
                    namespace: r.namespace.clone(),
                    deployment: r.deployment.clone(),
                    cpu_request: format_cpu(r.cpu_request_millicores),
                    cpu_limit: format_cpu(r.cpu_limit_millicores),
                    memory_request: format_bytes(r.memory_request_bytes),
                    memory_limit: format_bytes(r.memory_limit_bytes),
                    confidence: color_confidence(r.confidence),
                    status: color_status(&r.status),
                })
                .collect();

            let table = tabled::Table::new(rows)
                .with(tabled::settings::Style::rounded())
                .to_string();
            println!("{}", table);
            println!("\nTotal: {} recommendations", filtered.len());
        }
    }

    Ok(())
}

/// Get model versions
pub async fn get_models(client: &ApiClient, active_only: bool, format: OutputFormat) -> Result<()> {
    let result: ModelList = client.get("api/v1/models").await?;

    let filtered: Vec<_> = if active_only {
        result.models.into_iter().filter(|m| m.is_active).collect()
    } else {
        result.models
    };

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&filtered)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            if filtered.is_empty() {
                print_warning("No models found");
                return Ok(());
            }

            let rows: Vec<ModelRow> = filtered
                .iter()
                .map(|m| ModelRow {
                    version: m.version.clone(),
                    created_at: format_timestamp(&m.created_at),
                    accuracy: format!("{:.1}%", m.validation_accuracy * 100.0),
                    size: format_bytes(m.size_bytes as u64),
                    active: if m.is_active {
                        "✓".to_string()
                    } else {
                        "".to_string()
                    },
                    rollbacks: m.rollback_count.to_string(),
                })
                .collect();

            let table = tabled::Table::new(rows)
                .with(tabled::settings::Style::rounded())
                .to_string();
            println!("{}", table);
        }
    }

    Ok(())
}

/// Apply a recommendation
pub async fn apply_recommendation(
    client: &ApiClient,
    id: &str,
    dry_run: bool,
    format: OutputFormat,
) -> Result<()> {
    let path = format!("api/v1/recommendation/{}/apply", id);
    let request = ApplyRequest { dry_run };

    let response: crate::client::ApplyResponse = client.post(&path, &request).await?;

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&response)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            if dry_run {
                print_warning("Dry-run mode - no changes applied");
                println!("\nRecommendation: {}", id);
                println!("Status: {}", response.status);
                println!("Message: {}", response.message);

                if let Some(patch) = &response.yaml_patch {
                    println!("\nYAML Patch that would be applied:");
                    println!("---");
                    println!("{}", patch);
                }
            } else {
                print_success(&format!("Recommendation {} applied successfully", id));
                println!("Status: {}", response.status);
                println!("Message: {}", response.message);
            }
        }
    }

    Ok(())
}

/// Approve a recommendation
pub async fn approve_recommendation(
    client: &ApiClient,
    id: &str,
    approver: &str,
    reason: Option<String>,
    format: OutputFormat,
) -> Result<()> {
    let path = format!("api/v1/recommendation/{}/approve", id);
    let request = ApproveRequest {
        approver: approver.to_string(),
        reason,
    };

    let response: crate::client::ApproveResponse = client.post(&path, &request).await?;

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&response)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            print_success(&format!("Recommendation {} approved", id));
            println!("Status: {}", response.status);
            println!("Approver: {}", approver);
            println!("Message: {}", response.message);
        }
    }

    Ok(())
}

/// Truncate ID for display
fn truncate_id(id: &str) -> String {
    if id.len() > 8 {
        format!("{}...", &id[..8])
    } else {
        id.to_string()
    }
}

/// Format timestamp for display
fn format_timestamp(ts: &str) -> String {
    // Try to parse and format nicely, otherwise return as-is
    if let Ok(dt) = chrono::DateTime::parse_from_rfc3339(ts) {
        dt.format("%Y-%m-%d %H:%M").to_string()
    } else {
        ts.to_string()
    }
}
