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

use rocket::{
    http::Status,
    request::{self, FromRequest},
};

use base64::engine::general_purpose::STANDARD_NO_PAD;
use base64::Engine;
use jwt_simple::prelude::*;

// TOKEN
lazy_static::lazy_static! {
  pub static ref JWT_KEY: HS256Key = HS256Key::from_bytes(&STANDARD_NO_PAD.decode(std::env::var("JWT_KEY").unwrap()).unwrap());
}

pub struct ApiAuth {
    pub token: String,
}

#[derive(Serialize, Deserialize)]
pub struct CustomClaims {
    // Admin is the only supported scope
    scopes: Vec<String>,
}

#[derive(Debug)]
pub enum ApiError {
    Nil,
    NoAdminScope,
}

impl std::error::Error for ApiError {}
impl std::fmt::Display for ApiError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ApiError::Nil => write!(f, "Failed to verify token"),
            ApiError::NoAdminScope => write!(f, "Token has no admin scope"),
        }
    }
}

#[rocket::async_trait]
impl<'r> FromRequest<'r> for ApiAuth {
    type Error = ApiError;

    async fn from_request(req: &'r rocket::Request<'_>) -> request::Outcome<Self, Self::Error> {
        for token in req
            .headers()
            .get("Authorization")
            .filter_map(|token| token.strip_prefix("Bearer "))
        {
            let options = VerificationOptions::default();
            if let Ok(claims) = JWT_KEY.verify_token::<CustomClaims>(token, Some(options)) {
                return match claims.custom.scopes.contains(&"admin".to_string()) {
                    true => request::Outcome::Success(ApiAuth {
                        token: token.to_string(),
                    }),
                    false => request::Outcome::Error((Status::Forbidden, ApiError::NoAdminScope)),
                };
            };
        }
        request::Outcome::Error((Status::Forbidden, ApiError::Nil))
    }
}
