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

use crate::models::SearchFilter;
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

fn get_filter_string(filter: &str, filter_added: &mut bool) -> String {
    let mut filter_string: String;

    if *filter_added {
        filter_string = String::from(format!(" OR {filter} "));
    } else {
        filter_string = String::from(format!(" WHERE {filter} "));
        *filter_added = true;
    }

    filter_string.push_str("LIKE '%' || $1 || '%'");

    filter_string
}

async fn search_by_filters(
    mut db: Connection<RpmSqlite>,
    pkg_name: &str,
    filters: Vec<SearchFilter>,
) -> Result<serde_json::Value, Status> {
    let mut query: String;
    let is_provides: bool = filters.contains(&SearchFilter::Provides);

    if is_provides {
        query = "SELECT pkgId, packages.pkgKey, packages.name, provides.name AS providesName, arch FROM packages JOIN provides ON (provides.pkgKey = packages.pkgKey)".to_owned();
    } else {
        query = "SELECT pkgId, pkgKey, name, arch FROM packages".to_owned();
    }
    let mut filter_added = false;

    for filter in filters {
        match filter {
            SearchFilter::Name => {
                query.push_str(&get_filter_string("packages.name", &mut filter_added))
            }
            SearchFilter::Provides => {
                query.push_str(&get_filter_string("provides.name", &mut filter_added))
            }
        }
    }

    query.push_str(" --case-insensitive");

    sqlx::query_as::<_, SearchResult>(&query)
        .bind(pkg_name)
        .fetch_all(&mut **db)
        .await
        .map(|ret| serde_json::json!(ret))
        .map_err(|e| {
            println!("{e} ({})", &query);
            Status::InternalServerError
        })
}

#[get("/search?<q>&<filter>")]
pub async fn search(
    db: Connection<RpmSqlite>,
    q: &str,
    filter: Option<Vec<SearchFilter>>,
) -> Result<serde_json::Value, Status> {
    if let Some(filters) = filter {
        search_by_filters(db, q, filters).await
    } else {
        search_by_name(db, q).await
    }
}
