use crate::database::Connection;
use crate::database::RpmSqlite;

use crate::models::Package;

use rocket::http::Status;

#[get("/packages/<name>")]
pub async fn get_package(
    mut db: Connection<RpmSqlite>,
    name: &str,
) -> Result<serde_json::Value, Status> {
    sqlx::query_as::<_, Package>("SELECT * FROM packages WHERE name = $1")
        .bind(name)
        .fetch_all(&mut **db)
        .await
        .map(|ret| serde_json::json!(ret))
        .map_err(|_| Status::InternalServerError)
}
