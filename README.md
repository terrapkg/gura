# gura

## API

| Endpoint | Description |
|:-:|:-:|
| GET / | Redirect to Terra homepage |
| GET /_health | Check if server is running. Returns version |
| GET /api/packages/id/<id> | Get single package for ID |
| GET /api/packages/name/<name>?all | Get packages by name. This may include multiple packages. By default this returns a combined grouping of all related packages with the same name. Passing ?all will return an array containng all valid packages. |
| GET /api/search?q=>query>&filter=<filter> | Search for a package. Apply filters to refine search. Multiple filters can be applied. Supported filters: Name, Provides. If no filter is applied, Name is used. |
| POST /api/ci/repo_update | Update the currently running databse. Authenticate with a JWT Bearer token with 'admin' scope |

## License

Copyright (c) 2024 Fyra Labs

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with this program. If not, see https://www.gnu.org/licenses/.