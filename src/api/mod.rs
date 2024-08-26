use rocket::Route;

mod package;

pub fn routes() -> Vec<Route> {
    routes![package::get_package]
}
