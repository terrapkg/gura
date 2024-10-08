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

use rocket::response;

#[macro_use]
extern crate rocket;

mod api;
mod authentication;
mod database;
mod error;
mod models;

use database::Database;
use database::RpmSqlite;

#[get("/")]
fn index() -> response::Redirect {
    response::Redirect::to("https://terra.fyralabs.com/")
}

#[get("/_health")]
async fn health() -> String {
    format!("{} {}", env!("CARGO_PKG_NAME"), env!("CARGO_PKG_VERSION"))
}

#[launch]
fn rocket() -> _ {
    if let Err(e) = dotenv::dotenv() {
        println!("Ignoring .env: {e}")
    }
    assert!(std::env::var("JWT_KEY").is_ok(), "JWT_KEY cannot be empty");
    rocket::build()
        .attach(RpmSqlite::init())
        .mount("/", routes![index, health])
        .mount("/api", api::routes())
        .register("/", catchers![error::default_catcher])
}
