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

use crate::models::GroupedPackage;
use crate::models::Package;

use rocket::http::Status;

#[get("/packages/id/<id>")]
pub async fn get_package_by_id(
    mut db: Connection<RpmSqlite>,
    id: &str,
) -> Result<serde_json::Value, Status> {
    sqlx::query_as::<_, Package>("SELECT * FROM packages WHERE pkgId = $1")
        .bind(id)
        .fetch_one(&mut **db)
        .await
        .map(|ret| serde_json::json!(ret))
        .map_err(|_| Status::InternalServerError)
}

#[get("/packages/name/<name>?<all>")]
pub async fn get_package_by_name(
    mut db: Connection<RpmSqlite>,
    name: &str,
    all: bool, // This will show the details for both. If not included, a combined package is shown
) -> Result<serde_json::Value, Status> {
    let query = sqlx::query_as::<_, Package>("SELECT * FROM packages WHERE name = $1")
        .bind(name)
        .fetch_all(&mut **db)
        .await
        .map_err(|_| Status::InternalServerError)?;

    // TODO: make sure we're only returning two packages here
    // If not, filter all of them
    if all && (query[0] == query[1]) {
        Ok(serde_json::json!(query))
    } else {
        let first = query[0].clone();

        let ids = query.iter().map(|x| x.clone().id).collect::<Vec<String>>();
        let archs = query
            .iter()
            .map(|x| x.clone().arch)
            .collect::<Vec<String>>();

        let grouped = GroupedPackage {
            ids,
            archs,
            name: first.name,
            version: first.version,
            release: first.release,
            summary: first.summary,
            description: first.description,
            url: first.url,
        };
        Ok(serde_json::json!(grouped))
    }
}
