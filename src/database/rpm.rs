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

use crate::database::Database;

pub struct RpmSqlite(sqlx::SqlitePool);

impl From<sqlx::SqlitePool> for RpmSqlite {
    fn from(pool: sqlx::SqlitePool) -> Self {
        Self(pool)
    }
}

impl std::ops::Deref for RpmSqlite {
    type Target = sqlx::SqlitePool;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl std::ops::DerefMut for RpmSqlite {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

#[rocket::async_trait]
impl<'r> rocket::request::FromRequest<'r> for &'r RpmSqlite {
    type Error = ();

    async fn from_request(
        req: &'r rocket::request::Request<'_>,
    ) -> rocket::request::Outcome<Self, Self::Error> {
        match <RpmSqlite as Database>::fetch(req.rocket()) {
            Some(db) => rocket::outcome::Outcome::Success(db),
            None => {
                rocket::outcome::Outcome::Error((rocket::http::Status::InternalServerError, ()))
            }
        }
    }
}

impl rocket::Sentinel for RpmSqlite {
    fn abort(rocket: &rocket::Rocket<rocket::Ignite>) -> bool {
        <RpmSqlite as Database>::fetch(rocket).is_none()
    }
}

impl Database for RpmSqlite {
    type Pool = sqlx::SqlitePool;
    const NAME: &'static str = "RpmSqlite";

    fn init() -> super::db::Initializer<Self> {
        super::db::Initializer::with_name("RpmSqlite")
    }
}
