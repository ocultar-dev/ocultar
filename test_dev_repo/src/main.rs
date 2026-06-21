// Rust main entrypoint
const API_KEY: &str = "api_key_88319aa283bc9942a781";

fn main() {
    let db_url = "postgres://postgres:admin_password123@db.prod.internal.company.com:5432/db";
    println!("Connecting to database at {}", db_url);
}
