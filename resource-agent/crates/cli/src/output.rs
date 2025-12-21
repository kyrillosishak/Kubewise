//! Output formatting utilities

use clap::ValueEnum;
use colored::Colorize;
use serde::Serialize;
use tabled::{settings::Style, Table, Tabled};

/// Output format for CLI commands
#[derive(Debug, Clone, Copy, Default, ValueEnum)]
pub enum OutputFormat {
    /// Table format (default)
    #[default]
    Table,
    /// JSON format
    Json,
}

/// Print a table from a list of items
#[allow(dead_code)]
pub fn print_table<T: Tabled + Serialize>(items: &[T], format: OutputFormat) {
    match format {
        OutputFormat::Table => {
            if items.is_empty() {
                println!("{}", "No items found".yellow());
                return;
            }
            let table = Table::new(items).with(Style::rounded()).to_string();
            println!("{}", table);
        }
        OutputFormat::Json => {
            if let Ok(json) = serde_json::to_string_pretty(&items) {
                println!("{}", json);
            }
        }
    }
}

/// Print a success message
pub fn print_success(message: &str) {
    println!("{} {}", "✓".green().bold(), message);
}

/// Print an error message
#[allow(dead_code)]
pub fn print_error(message: &str) {
    eprintln!("{} {}", "✗".red().bold(), message);
}

/// Print a warning message
pub fn print_warning(message: &str) {
    println!("{} {}", "⚠".yellow().bold(), message);
}

/// Print an info message
pub fn print_info(message: &str) {
    println!("{} {}", "ℹ".blue().bold(), message);
}

/// Format bytes as human-readable string
pub fn format_bytes(bytes: u64) -> String {
    const KB: u64 = 1024;
    const MB: u64 = KB * 1024;
    const GB: u64 = MB * 1024;

    if bytes >= GB {
        format!("{:.2}Gi", bytes as f64 / GB as f64)
    } else if bytes >= MB {
        format!("{:.2}Mi", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{:.2}Ki", bytes as f64 / KB as f64)
    } else {
        format!("{}B", bytes)
    }
}

/// Format millicores as human-readable string
pub fn format_cpu(millicores: u32) -> String {
    if millicores >= 1000 {
        format!("{:.1}", millicores as f64 / 1000.0)
    } else {
        format!("{}m", millicores)
    }
}

/// Format confidence as percentage
pub fn format_confidence(confidence: f32) -> String {
    format!("{:.0}%", confidence * 100.0)
}

/// Format currency
pub fn format_currency(amount: f64, currency: &str) -> String {
    match currency {
        "USD" => format!("${:.2}", amount),
        "EUR" => format!("€{:.2}", amount),
        "GBP" => format!("£{:.2}", amount),
        _ => format!("{:.2} {}", amount, currency),
    }
}

/// Color status based on value
pub fn color_status(status: &str) -> String {
    match status.to_lowercase().as_str() {
        "pending" => status.yellow().to_string(),
        "approved" => status.blue().to_string(),
        "applied" => status.green().to_string(),
        "rolled_back" => status.red().to_string(),
        "healthy" | "running" => status.green().to_string(),
        "degraded" | "warning" => status.yellow().to_string(),
        "unhealthy" | "error" | "failed" => status.red().to_string(),
        _ => status.to_string(),
    }
}

/// Color confidence based on value
pub fn color_confidence(confidence: f32) -> String {
    let formatted = format_confidence(confidence);
    if confidence >= 0.8 {
        formatted.green().to_string()
    } else if confidence >= 0.6 {
        formatted.yellow().to_string()
    } else {
        formatted.red().to_string()
    }
}
