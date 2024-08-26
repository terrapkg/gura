use std::marker::PhantomData;
use std::ops::{Deref, DerefMut};

use rocket::fairing::{self, Fairing, Info, Kind};
use rocket::http::Status;
use rocket::request::{FromRequest, Outcome, Request};
use rocket::{error, info_, Build, Ignite, Orbit, Phase, Rocket, Sentinel};

use super::DbPool;

pub trait Database:
    From<Self::Pool> + DerefMut<Target = Self::Pool> + Send + Sync + 'static
{
    type Pool: DbPool;
    const NAME: &'static str;

    fn init() -> Initializer<Self> {
        Initializer::new()
    }

    fn fetch<P: Phase>(rocket: &Rocket<P>) -> Option<&Self> {
        if let Some(db) = rocket.state() {
            return Some(db);
        }

        let dbtype = std::any::type_name::<Self>();
        error!("Attempted to fetch unattached database `{}`.", dbtype);
        info_!(
            "`{}{}` fairing must be attached prior to using this database.",
            dbtype,
            "::init()"
        );
        None
    }
}

pub struct Initializer<D: Database>(Option<&'static str>, PhantomData<fn() -> D>);
pub struct Connection<D: Database>(<D::Pool as DbPool>::Connection);

impl<D: Database> Initializer<D> {
    pub fn new() -> Self {
        Self(None, std::marker::PhantomData)
    }

    pub fn with_name(name: &'static str) -> Self {
        Self(Some(name), std::marker::PhantomData)
    }
}

impl<D: Database> Connection<D> {
    pub fn into_inner(self) -> <D::Pool as DbPool>::Connection {
        self.0
    }
}

#[rocket::async_trait]
impl<'r, D: Database> FromRequest<'r> for Connection<D> {
    type Error = Option<<D::Pool as DbPool>::Error>;

    async fn from_request(req: &'r Request<'_>) -> Outcome<Self, Self::Error> {
        match D::fetch(req.rocket()) {
            Some(db) => match db.get().await {
                Ok(conn) => Outcome::Success(Connection(conn)),
                Err(e) => Outcome::Error((Status::ServiceUnavailable, Some(e))),
            },
            None => Outcome::Error((Status::InternalServerError, None)),
        }
    }
}

#[rocket::async_trait]
impl<D: Database> Fairing for Initializer<D> {
    fn info(&self) -> Info {
        Info {
            name: self.0.unwrap_or(std::any::type_name::<Self>()),
            kind: Kind::Ignite | Kind::Shutdown,
        }
    }

    async fn on_ignite(&self, rocket: Rocket<Build>) -> fairing::Result {
        // let workers: usize = rocket
        //     .figment()
        //     .extract_inner(rocket::Config::WORKERS)
        //     .unwrap_or_else(|_| rocket::Config::default().workers);

        // let figment = rocket
        //     .figment()
        //     .focus(&format!("databases.{}", D::NAME))
        //     .join(Serialized::default("max_connections", workers * 4))
        //     .join(Serialized::default("connect_timeout", 5));

        match <D::Pool>::init().await {
            Ok(pool) => Ok(rocket.manage(D::from(pool))),
            Err(e) => {
                error!("failed to initialize database: {}", e);
                Err(rocket)
            }
        }
    }

    async fn on_shutdown(&self, rocket: &Rocket<Orbit>) {
        if let Some(db) = D::fetch(rocket) {
            db.close().await;
        }
    }
}

impl<D: Database> Sentinel for Connection<D> {
    fn abort(rocket: &Rocket<Ignite>) -> bool {
        D::fetch(rocket).is_none()
    }
}

impl<D: Database> Deref for Connection<D> {
    type Target = <D::Pool as DbPool>::Connection;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl<D: Database> DerefMut for Connection<D> {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}
