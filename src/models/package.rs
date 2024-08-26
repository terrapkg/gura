use serde::Serialize;
use sqlx::FromRow;

#[derive(FromRow, Serialize, Debug)]
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

impl PartialEq for Package {
    fn eq(&self, other: &Self) -> bool {
        (self.name == other.name)
            && (self.version == other.version)
            && (self.release == other.release)
    }
}
