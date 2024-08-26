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

use rocket::FromFormField;
use serde::Serialize;
use sqlx::FromRow;

#[derive(Debug, PartialEq, FromFormField)]
pub enum SearchFilter {
    Name,
    Provides,
}

#[derive(FromRow, Serialize, Debug)]
pub struct SearchResult {
    #[sqlx(rename = "pkgId")]
    pub id: String,
    #[sqlx(rename = "pkgKey")]
    pub key: i32,
    pub name: String,
    pub arch: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    #[sqlx(default, rename = "providesName")]
    pub provides: Option<String>,
}
