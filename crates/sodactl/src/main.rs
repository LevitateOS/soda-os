use clap::{Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "sodactl", version, about = "Administer Soda OS")]
struct Cli {
    #[arg(long, default_value = "/run/soda/sodad.sock")]
    socket: String,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Health,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    match cli.command {
        Command::Health => println!("sodad socket: {}", cli.socket),
    }
    Ok(())
}
