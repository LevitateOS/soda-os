use clap::Parser;

#[derive(Debug, Parser)]
#[command(name = "soda-ssh", version, about = "Enter a Soda project worktree")]
struct Cli {
    #[arg(long)]
    actor: String,
    #[arg(long)]
    worktree: String,
}

fn main() {
    let cli = Cli::parse();
    println!("actor={} worktree={}", cli.actor, cli.worktree);
}
