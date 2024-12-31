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
        .map_err(|e| {
            println!("{:?}", e);
            match e {
                sqlx::Error::RowNotFound => Status::NotFound,
                _ => Status::InternalServerError,
            }
        })
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
        .map_err(|e| {
            println!("{:?}", e);
            match e {
                sqlx::Error::RowNotFound => Status::NotFound,
                _ => Status::InternalServerError,
            }
        })?;

    if query.is_empty() {
        return Err(Status::NotFound);
    }

    // Don't combine packages if all is set AND there's only one package AND all the packages are the same.
    if all && (query.len() < 2 || query.windows(2).all(|q| q[0] == q[1])) {
        return Ok(serde_json::json!(query));
    }

    let first = query[0].clone();

    let ids = query.iter().map(|x| x.clone().id).collect::<Vec<String>>();
    let archs = query
        .iter()
        .map(|x| x.clone().arch)
        .collect::<Vec<String>>();
    let mut version_differ = false;
    let mut release_differ = false;

    if query.len() > 1 {
        version_differ = !query.windows(2).all(|q| q[0].version == q[1].version);
        release_differ = !query.windows(2).all(|q| q[0].release == q[1].release);
    }

    let grouped = GroupedPackage {
        ids,
        archs,
        name: first.name, // Is always the same
        version: if version_differ {
            "Versions Differ".to_string()
        } else {
            first.version
        },
        release: if release_differ {
            "Releases Differ".to_string()
        } else {
            first.release
        },
        summary: first.summary,
        description: first.description,
        url: first.url,
        license: first.license,
        category: first.category,
        packager: first.packager,
    };
    Ok(serde_json::json!(grouped))
}
