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

use rocket::futures::StreamExt;
use rocket::tokio::fs::File;
use rocket::tokio::io::BufWriter;

use crate::models::Repodata;

async fn get_file_url(repodata: String) -> Result<String, Box<dyn std::error::Error>> {
    let xml_stream = reqwest::get(repodata).await?.text().await?;

    let xml: Repodata = serde_xml_rs::from_str(&xml_stream).unwrap();

    if let Some(loc) = xml.data.iter().find(|data| data._type == "primary_db") {
        let href = loc.location.href.clone().replace("repodata/", "");
        return Ok(href);
    }

    Err("File Not Found".into())
}

pub async fn download() -> Result<String, Box<dyn std::error::Error>> {
    const DOWNLOAD_URL: &str = "https://repos.fyralabs.com/terra41/repodata/";

    let file_url = get_file_url(DOWNLOAD_URL.to_owned() + "/repomd.xml").await?;

    let mut stream = reqwest::get(DOWNLOAD_URL.to_owned() + &file_url)
        .await?
        .bytes_stream();

    let mut filename = std::env::temp_dir();
    filename.push("DATABASE.sqlite.xz");

    let mut out = BufWriter::new(File::create(filename.clone()).await?);

    while let Some(chunk) = stream.next().await {
        rocket::tokio::io::copy(&mut chunk?.as_ref(), &mut out).await?;
    }

    filename
        .into_os_string()
        .into_string()
        .map_err(|_| std::io::Error::new(std::io::ErrorKind::Other, "oh nyo").into())
}
