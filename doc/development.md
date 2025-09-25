# Development

This document provides information on how to develop and contribute to the project.

## Definitions

- Package: a downstream repository package.
  - Different versions, architectures, and distributions constitute different packages.
  - `libexample`, `libexample-devel`, and `example-bash-completion` are all separate packages.
- Stream: an upstream. This upstream may have different mirrors, and we track streams to determine the latest version.
- TRACE: information (currently just URLs) for `nobori` to guess its upstream.
- Swim: to fetch stream information from a forge.

## Project Structure

The project is organized into several packages, each with specific responsibilities and dependencies:

- **kudari**: Fetch and manage downstream repository packages
  - Depend on `repomd`, `nobori`, `db`
  - `kudari` needs to insert new packages using `nobori.RegPkg()` (strm.go), which handles stream grouping.
- **nobori**: Handle upstream project tracking
  - Depend on `repomd`, `db`
  - `nobori/TRACE.go` needs to obtain package metadata to assign the correct stream. `repomd` contains the structure of the repository metadata for decoding.
- **repomd**: Types for repository metadata
- **db**: Database access and abstraction
- **api**: Backend API implementation
  - Depend on `db`

Each package directory may contain additional information in their separate README.md files.

## Writing Documentation

Code documentations should be written in markdown. If the first word is a verb, omit the extra `s` and fullstop.