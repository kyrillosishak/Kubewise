//! Container Resource Predictor CLI
//!
//! A command-line tool for querying recommendations, viewing costs,
//! and debugging the container resource predictor system.

mod client;
mod commands;
mod config;
mod output;

use anyhow::Result;
use clap::{Parser, Subcommand};
use commands::{costs, debug, recommendations};

/// Container Resource Predictor CLI
#[derive(Parser)]
#[command(name = "crp")]
#[command(author, version, about = "CLI for Container Resource Predictor", long_about = None)]
pub struct Cli {
    /// API endpoint URL (can also be set via CRP_API_URL env var)
    #[arg(long, env = "CRP_API_URL", default_value = "http://localhost:8080")]
    pub api_url: String,

    /// Path to kubeconfig file (uses default if not specified)
    #[arg(long, env = "KUBECONFIG")]
    pub kubeconfig: Option<String>,

    /// Output format
    #[arg(long, short, default_value = "table")]
    pub format: output::OutputFormat,

    /// Enable verbose output
    #[arg(long, short)]
    pub verbose: bool,

    #[command(subcommand)]
    pub command: Commands,
}

#[derive(Subcommand)]
pub enum Commands {
    /// Get resource recommendations
    #[command(subcommand)]
    Get(GetCommands),

    /// Apply a recommendation
    Apply {
        /// Recommendation ID to apply
        id: String,

        /// Perform a dry-run without applying changes
        #[arg(long)]
        dry_run: bool,
    },

    /// Approve a recommendation for application
    Approve {
        /// Recommendation ID to approve
        id: String,

        /// Approver name
        #[arg(long, default_value = "cli-user")]
        approver: String,

        /// Reason for approval
        #[arg(long)]
        reason: Option<String>,
    },

    /// View cost analysis and savings
    #[command(subcommand)]
    Costs(CostsCommands),

    /// Debug and troubleshooting commands
    #[command(subcommand)]
    Debug(DebugCommands),
}

#[derive(Subcommand)]
pub enum GetCommands {
    /// Get recommendations
    Recommendations {
        /// Filter by namespace
        #[arg(long, short)]
        namespace: Option<String>,

        /// Filter by deployment name
        #[arg(long, short)]
        deployment: Option<String>,

        /// Filter by status (pending, approved, applied, rolled_back)
        #[arg(long)]
        status: Option<String>,
    },

    /// Get model versions
    Models {
        /// Show only active model
        #[arg(long)]
        active_only: bool,
    },
}

#[derive(Subcommand)]
pub enum CostsCommands {
    /// Show cost analysis
    Show {
        /// Filter by namespace (shows cluster-wide if not specified)
        #[arg(long, short)]
        namespace: Option<String>,
    },

    /// Show savings report
    Savings {
        /// Time period (e.g., 30d, 90d, 1y)
        #[arg(long, default_value = "30d")]
        since: String,
    },
}

#[derive(Subcommand)]
pub enum DebugCommands {
    /// Show prediction history for a deployment
    Predictions {
        /// Deployment name (format: namespace/deployment or just deployment)
        deployment: String,
    },

    /// Show agent status on a node
    Agent {
        /// Node name
        node: String,
    },

    /// Export metrics data
    Export {
        /// Time period to export (e.g., 1h, 24h, 7d)
        #[arg(long, default_value = "24h")]
        since: String,

        /// Output file path
        #[arg(long, short)]
        output: Option<String>,

        /// Namespace filter
        #[arg(long, short)]
        namespace: Option<String>,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Initialize client
    let client = client::ApiClient::new(&cli.api_url)?;

    // Execute command
    match cli.command {
        Commands::Get(get_cmd) => match get_cmd {
            GetCommands::Recommendations {
                namespace,
                deployment,
                status,
            } => {
                recommendations::get_recommendations(&client, namespace, deployment, status, cli.format).await?;
            }
            GetCommands::Models { active_only } => {
                recommendations::get_models(&client, active_only, cli.format).await?;
            }
        },
        Commands::Apply { id, dry_run } => {
            recommendations::apply_recommendation(&client, &id, dry_run, cli.format).await?;
        }
        Commands::Approve { id, approver, reason } => {
            recommendations::approve_recommendation(&client, &id, &approver, reason, cli.format).await?;
        }
        Commands::Costs(costs_cmd) => match costs_cmd {
            CostsCommands::Show { namespace } => {
                costs::show_costs(&client, namespace, cli.format).await?;
            }
            CostsCommands::Savings { since } => {
                costs::show_savings(&client, &since, cli.format).await?;
            }
        },
        Commands::Debug(debug_cmd) => match debug_cmd {
            DebugCommands::Predictions { deployment } => {
                debug::show_predictions(&client, &deployment, cli.format).await?;
            }
            DebugCommands::Agent { node } => {
                debug::show_agent_status(&client, &node, cli.format).await?;
            }
            DebugCommands::Export { since, output, namespace } => {
                debug::export_metrics(&client, &since, output, namespace, cli.format).await?;
            }
        },
    }

    Ok(())
}
