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

use std::env;

use base64::{engine::general_purpose::STANDARD_NO_PAD, Engine};
use jwt_simple::prelude::*;

fn main() {
    let task = env::args().nth(1);
    match task.as_deref() {
        Some("generate-jwt-key") => generate_jwt_key(),
        _ => {
            eprintln!(
                "Tasks:
generate-jwt-key    generates a JWT key for use in the JWT_KEY environment variable
"
            )
        }
    };
}

fn generate_jwt_key() {
    let key = HS256Key::generate();
    println!("{}", STANDARD_NO_PAD.encode(key.to_bytes()));
}
