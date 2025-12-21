//! Cost-related CLI commands

use anyhow::Result;
use colored::Colorize;
use tabled::Tabled;

use crate::client::{ApiClient, CostAnalysis, SavingsReport};
use crate::output::{format_currency, OutputFormat};

/// Row for savings by month table
#[derive(Tabled)]
struct MonthlySavingRow {
    #[tabled(rename = "Month")]
    month: String,
    #[tabled(rename = "Savings")]
    savings: String,
}

/// Row for savings by team table
#[derive(Tabled)]
struct TeamSavingRow {
    #[tabled(rename = "Team")]
    team: String,
    #[tabled(rename = "Savings")]
    savings: String,
}

/// Show cost analysis
pub async fn show_costs(
    client: &ApiClient,
    namespace: Option<String>,
    format: OutputFormat,
) -> Result<()> {
    let path = match &namespace {
        Some(ns) => format!("api/v1/costs/{}", ns),
        None => "api/v1/costs".to_string(),
    };

    let result: CostAnalysis = client.get(&path).await?;

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&result)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            println!("{}", "Cost Analysis".bold());
            println!("{}", "=".repeat(50));

            if let Some(ns) = &result.namespace {
                println!("Namespace:              {}", ns.cyan());
            } else {
                println!("Scope:                  {}", "Cluster-wide".cyan());
            }

            println!("Deployments:            {}", result.deployment_count);
            println!();

            println!("{}", "Monthly Costs".bold());
            println!("{}", "-".repeat(50));
            println!(
                "Current:                {}",
                format_currency(result.current_monthly_cost, &result.currency)
            );
            println!(
                "Recommended:            {}",
                format_currency(result.recommended_monthly_cost, &result.currency).green()
            );
            println!();

            let savings_str = format_currency(result.potential_savings, &result.currency);
            let savings_pct = if result.current_monthly_cost > 0.0 {
                (result.potential_savings / result.current_monthly_cost) * 100.0
            } else {
                0.0
            };

            println!(
                "{} {} ({:.1}%)",
                "Potential Savings:".bold(),
                savings_str.green().bold(),
                savings_pct
            );

            println!();
            println!(
                "Last updated: {}",
                format_timestamp(&result.last_updated).dimmed()
            );
        }
    }

    Ok(())
}

/// Show savings report
pub async fn show_savings(client: &ApiClient, since: &str, format: OutputFormat) -> Result<()> {
    let path = format!("api/v1/savings?since={}", since);
    let result: SavingsReport = client.get(&path).await?;

    match format {
        OutputFormat::Json => {
            let json = serde_json::to_string_pretty(&result)?;
            println!("{}", json);
        }
        OutputFormat::Table => {
            println!("{}", "Savings Report".bold());
            println!("{}", "=".repeat(50));
            println!("Period:                 {}", result.period);
            println!(
                "{}  {}",
                "Total Savings:".bold(),
                format_currency(result.total_savings, &result.currency)
                    .green()
                    .bold()
            );
            println!();

            // Monthly breakdown
            if !result.savings_by_month.is_empty() {
                println!("{}", "Savings by Month".bold());
                println!("{}", "-".repeat(50));

                let rows: Vec<MonthlySavingRow> = result
                    .savings_by_month
                    .iter()
                    .map(|m| MonthlySavingRow {
                        month: m.month.clone(),
                        savings: format_currency(m.savings, &result.currency),
                    })
                    .collect();

                let table = tabled::Table::new(rows)
                    .with(tabled::settings::Style::rounded())
                    .to_string();
                println!("{}", table);
                println!();
            }

            // Team breakdown
            if let Some(teams) = &result.savings_by_team {
                if !teams.is_empty() {
                    println!("{}", "Savings by Team".bold());
                    println!("{}", "-".repeat(50));

                    let rows: Vec<TeamSavingRow> = teams
                        .iter()
                        .map(|t| TeamSavingRow {
                            team: t.team.clone(),
                            savings: format_currency(t.savings, &result.currency),
                        })
                        .collect();

                    let table = tabled::Table::new(rows)
                        .with(tabled::settings::Style::rounded())
                        .to_string();
                    println!("{}", table);
                }
            }
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
