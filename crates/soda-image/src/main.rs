use std::path::PathBuf;

use clap::{Parser, Subcommand};
use soda_core::DistroSpec;

#[derive(Debug, Parser)]
#[command(name = "soda-image", version, about = "Build Soda OS artifacts")]
struct Cli {
    #[arg(long, default_value = "distro/soda.toml")]
    spec: PathBuf,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Check,
    Rpm,
    Iso,
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    let spec = DistroSpec::load(&cli.spec)?;
    match cli.command {
        Command::Check => println!(
            "{} {} spec is valid",
            spec.identity.name, spec.identity.version
        ),
        Command::Rpm => println!("RPM build scaffold for {}", spec.identity.name),
        Command::Iso => println!("ISO build scaffold for {}", spec.identity.name),
    }
    Ok(())
}
