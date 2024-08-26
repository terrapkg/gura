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

use super::{download::download, error::Error};
use std::time::Duration;

use super::download::get_latest_database;

type Options<D> = <<D as sqlx::Database>::Connection as sqlx::Connection>::Options;

#[rocket::async_trait]
pub trait DbPool: Sized + Send + Sync + 'static {
    type Connection;
    type Error: std::error::Error;

    async fn init() -> Result<Self, Self::Error>;

    async fn reload(&self) -> Result<(), Self::Error>;

    async fn get(&self) -> Result<Self::Connection, Self::Error>;

    async fn close(&self);
}

#[rocket::async_trait]
impl<D: sqlx::Database> DbPool for sqlx::Pool<D> {
    type Error = Error<sqlx::Error>;
    type Connection = sqlx::pool::PoolConnection<D>;

    async fn init() -> Result<Self, Self::Error> {
        // FIXME: Check if the database exists, and if it doesn't, download it. Possibly have reqwests cache the request?
        let database_name = download().await.map_err(|_| {
            Error::Init(sqlx::Error::Protocol(
                "Unable to get latest database".to_owned(),
            ))
        })?;

        sqlx::pool::PoolOptions::new()
            .max_connections(4) // FIXME: determine a proper value for this
            .acquire_timeout(Duration::from_secs(5))
            .connect(&database_name)
            .await
            .map_err(Error::Init)
    }

    // FIXME: compare the database names and only update if one is newer
    async fn reload(&self) -> Result<(), Self::Error> {
        let database_name = download().await.map_err(|_| {
            Error::Init(sqlx::Error::Protocol(
                "Unable to get latest database".to_owned(),
            ))
        })?;

        let opts = database_name.parse::<Options<D>>().map_err(Error::Init)?;

        self.set_connect_options(opts);

        Ok(())
    }

    async fn get(&self) -> Result<Self::Connection, Self::Error> {
        self.acquire().await.map_err(Error::Get)
    }

    async fn close(&self) {
        <sqlx::Pool<D>>::close(self).await;
    }
}
