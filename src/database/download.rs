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
use std::fs::File;
use std::io::BufWriter;

use crate::models::Repodata;

async fn get_file_url(repodata: String) -> Result<(String, String), Box<dyn std::error::Error>> {
    let xml_stream = reqwest::get(repodata).await?.text().await?;

    let xml: Repodata = serde_xml_rs::from_str(&xml_stream).unwrap();

    if let Some(loc) = xml.data.iter().find(|data| data._type == "primary_db") {
        let href = loc.location.href.clone().replace("repodata/", "");
        return Ok((href, xml.revision));
    }

    Err("File Not Found".into())
}

/// Returns the path to the downloaded database.
pub async fn download() -> Result<String, Box<dyn std::error::Error>> {
    const DOWNLOAD_URL: &str = "https://repos.fyralabs.com/terra41/repodata/";

    let (file_url, revision) = get_file_url(DOWNLOAD_URL.to_owned() + "/repomd.xml").await?;

    let mut stream = reqwest::get(DOWNLOAD_URL.to_owned() + &file_url)
        .await?
        .bytes_stream();

    let mut filename = std::env::temp_dir();
    filename.push(format!("primary-{}.sqlite", revision));

    let out = BufWriter::new(File::create(filename.clone())?);

    let mut xz_decode = xz::write::XzDecoder::new(out);

    while let Some(chunk) = stream.next().await {
        std::io::copy(&mut chunk?.as_ref(), &mut xz_decode)?;
    }

    filename
        .into_os_string()
        .into_string()
        .map_err(|_| std::io::Error::new(std::io::ErrorKind::Other, "oh nyo").into())
}
