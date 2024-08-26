// gura -- Terra Package Server
//
// This file is a part of gura
//
// gura is free software: you can redistribute it and/or modify it under the terms of
// the GNU General Public License as published by the Free Software Foundation, either
// version 3 of the License, or (at your option) any later version.
//
// gura is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
// without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with gura.
// If not, see <https://www.gnu.org/licenses/>.

use crate::database::Connection;
use crate::database::RpmSqlite;

use crate::models::SearchResult;

use rocket::http::Status;

async fn search_by_name(
    mut db: Connection<RpmSqlite>,
    pkg_name: &str,
) -> Result<serde_json::Value, Status> {
    sqlx::query_as::<_, SearchResult>(
        "SELECT pkgId, pkgKey, name, arch
        FROM packages
        WHERE name
        LIKE '%' || $1 || '%'
        --case-insensitive",
    )
    .bind(pkg_name)
    .fetch_all(&mut **db)
    .await
    .map(|ret| serde_json::json!(ret))
    .map_err(|e| {
        println!("{e}");
        Status::InternalServerError
    })
}

#[get("/search?<q>")]
pub async fn search(db: Connection<RpmSqlite>, q: &str) -> Result<serde_json::Value, Status> {
    search_by_name(db, q).await
}
