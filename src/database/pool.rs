use super::error::Error;
use std::time::Duration;

#[rocket::async_trait]
pub trait DbPool: Sized + Send + Sync + 'static {
    type Connection;
    type Error: std::error::Error;

    async fn init() -> Result<Self, Self::Error>;

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

    async fn get(&self) -> Result<Self::Connection, Self::Error> {
        self.acquire().await.map_err(Error::Get)
    }

    async fn close(&self) {
        <sqlx::Pool<D>>::close(self).await;
    }
}
