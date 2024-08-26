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

use std::fmt;

#[derive(Debug)]
pub enum Error<A, B = A> {
    Init(A),
    Get(B),
}

impl<A: fmt::Display, B: fmt::Display> fmt::Display for Error<A, B> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Init(e) => write!(f, "failed to initialize database: {}", e),
            Error::Get(e) => write!(f, "failed to get db connection: {}", e),
        }
    }
}

impl<A, B> std::error::Error for Error<A, B>
where
    A: fmt::Debug + fmt::Display,
    B: fmt::Debug + fmt::Display,
{
}
