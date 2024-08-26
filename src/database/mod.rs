pub mod db;
mod error;
mod pool;

use pool::DbPool;

pub use db::Database;
