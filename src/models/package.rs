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

use serde::Serialize;
use sqlx::FromRow;

#[derive(FromRow, Serialize, Debug, Clone)]
pub struct Package {
    #[sqlx(rename = "pkgId")]
    pub id: String,
    #[sqlx(rename = "pkgKey")]
    pub key: i32,
    pub name: String,
    pub arch: String,
    pub version: String,
    pub epoch: String,
    pub release: String,
    pub summary: String,
    pub description: String,
    pub url: String,
    pub time_file: i32,
    pub time_build: i32,
}

#[derive(Serialize, Debug)]
pub struct GroupedPackage {
    pub ids: Vec<String>, // Return both package IDs in case more details is wanted about a specific variant
    pub archs: Vec<String>,

    pub name: String,
    pub version: String,
    pub release: String,
    pub summary: String,
    pub description: String,
    pub url: String,
}

impl PartialEq for Package {
    fn eq(&self, other: &Self) -> bool {
        (self.name == other.name)
            && (self.version == other.version)
            && (self.release == other.release)
    }
}
