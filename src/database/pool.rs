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

use super::error::Error;
use std::time::Duration;

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
        sqlx::pool::PoolOptions::new()
            .max_connections(4) // FIXME: determine a proper value for this
            .acquire_timeout(Duration::from_secs(5))
            .connect("primary.sqlite")
            .await
            .map_err(Error::Init)
    }

    async fn reload(&self) -> Result<(), Self::Error> {
        let opts = "".parse::<Options<D>>().map_err(Error::Init)?;

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
