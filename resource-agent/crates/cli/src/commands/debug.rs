//! Debug and troubleshooting CLI commands

use anyhow::Result;
use colored::Colorize;
use tabled::Tabled;

use crate::client::{AgentStatus, ApiClient, MetricsExport, PredictionHistory};
use crate::output::{
    color_confidence, color_status, format_bytes, format_cpu, print_info, print_success,
    print_warning, OutputFormat,
};

/// Row for predictions table
#[derive(Tabled)]
struct PredictionRow {
    #[tabled(rename = "Timestamp")]
    timestamp: String,
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
    #[tabled(rename = "Model")]
    model: String,
}

/// Show prediction history for a deployment
pub async fn show_predictions(
    client: &ApiClient,
    deployment: &str,
    format: OutputFormat,
) -> Result<()> {
    let path = format!("api/v1/debug/predictions/{}", deployment);
    let result: PredictionHistory = client.get(&path).await?;

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&result)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            println!("{}", "Prediction History".bold());
            println!("{}", "=".repeat(60));
            println!("Deployment: {}", result.deployment.cyan());
            if !result.namespace.is_empty() {
                println!("Namespace:  {}", result.namespace.cyan());
            }
            println!();

            if result.predictions.is_empty() {
                print_warning("No predictions found for this deployment");
                return Ok(());
            }

            let rows: Vec<PredictionRow> = result
                .predictions
                .iter()
                .map(|p| PredictionRow {
                    timestamp: format_timestamp(&p.timestamp),
                    cpu_request: format_cpu(p.cpu_request_millicores),
                    cpu_limit: format_cpu(p.cpu_limit_millicores),
                    memory_request: format_bytes(p.memory_request_bytes),
                    memory_limit: format_bytes(p.memory_limit_bytes),
                    confidence: color_confidence(p.confidence),
                    model: p.model_version.clone(),
                })
                .collect();

            let table = tabled::Table::new(rows)
                .with(tabled::settings::Style::rounded())
                .to_string();
            println!("{}", table);
            println!("\nTotal: {} predictions", result.predictions.len());
        }
    }

    Ok(())
}

/// Show agent status on a node
pub async fn show_agent_status(client: &ApiClient, node: &str, format: OutputFormat) -> Result<()> {
    // Try to get agent status from the API
    // Note: This endpoint may need to be added to the API
    let path = format!("api/v1/agents/{}", node);

    // For now, we'll try to get the status and handle the case where
    // the endpoint doesn't exist yet
    let result: Result<AgentStatus, _> = client.get(&path).await;

    match result {
        Ok(status) => match format {
            OutputFormat::Json => {
                let json = serde_json::to_string_pretty(&status)?;
                println!("{}", json);
            }
            OutputFormat::Table => {
                println!("{}", "Agent Status".bold());
                println!("{}", "=".repeat(50));
                println!("Node:                   {}", status.node.cyan());
                println!("Status:                 {}", color_status(&status.status));
                println!(
                    "Last Seen:              {}",
                    format_timestamp(&status.last_seen)
                );
                println!();
                println!("{}", "Metrics".bold());
                println!("{}", "-".repeat(50));
                println!("Containers Monitored:   {}", status.containers_monitored);
                println!("Model Version:          {}", status.model_version);
                println!(
                    "Buffer Size:            {}",
                    format_bytes(status.buffer_size_bytes)
                );
                println!();
                println!("{}", "Performance".bold());
                println!("{}", "-".repeat(50));
                println!(
                    "Collection Latency:     {:.2}ms",
                    status.collection_latency_ms
                );
                println!(
                    "Prediction Latency:     {:.2}ms",
                    status.prediction_latency_ms
                );
            }
        },
        Err(_) => {
            // Endpoint might not exist, provide helpful message
            print_warning(&format!(
                "Could not retrieve agent status for node '{}'",
                node
            ));
            print_info("The agent status endpoint may not be available.");
            print_info("You can check agent health directly via:");
            println!("  kubectl get pods -n crp-system -l app=resource-agent -o wide");
            println!("  kubectl logs -n crp-system -l app=resource-agent --tail=50");
        }
    }

    Ok(())
}

/// Export metrics data
pub async fn export_metrics(
    client: &ApiClient,
    since: &str,
    output: Option<String>,
    namespace: Option<String>,
    format: OutputFormat,
) -> Result<()> {
    let mut path = format!("api/v1/metrics/export?since={}", since);
    if let Some(ns) = &namespace {
        path.push_str(&format!("&namespace={}", ns));
    }

    // Try to get metrics export
    let result: Result<MetricsExport, _> = client.get(&path).await;

    match result {
        Ok(export) => {
            let json = serde_json::to_string_pretty(&export)?;

            if let Some(output_path) = output {
                std::fs::write(&output_path, &json)?;
                print_success(&format!("Metrics exported to {}", output_path));
                println!("Exported {} metric entries", export.metrics.len());
            } else {
                match format {
                    OutputFormat::Json => {
                        println!("{}", json);
                    }
                    OutputFormat::Table => {
                        println!("{}", "Metrics Export".bold());
                        println!("{}", "=".repeat(50));
                        println!("Period:     {}", export.since);
                        if let Some(ns) = &export.namespace {
                            println!("Namespace:  {}", ns);
                        }
                        println!("Entries:    {}", export.metrics.len());
                        println!();
                        print_info("Use --output <file> to save to a file");
                        print_info("Use --format json to see full data");
                    }
                }
            }
        }
        Err(_) => {
            // Endpoint might not exist, provide helpful message
            print_warning("Could not export metrics");
            print_info("The metrics export endpoint may not be available.");
            print_info("You can export metrics directly from TimescaleDB:");
            println!();
            println!("  psql -h <db-host> -U <user> -d predictor -c \\");
            println!("    \"COPY (SELECT * FROM container_metrics WHERE time > NOW() - INTERVAL '{}') TO STDOUT WITH CSV HEADER\"", since);
        }
    }

    Ok(())
}

/// Format timestamp for display
fn format_timestamp(ts: &str) -> String {
    if let Ok(dt) = chrono::DateTime::parse_from_rfc3339(ts) {
        dt.format("%Y-%m-%d %H:%M:%S").to_string()
    } else {
        ts.to_string()
    }
}
