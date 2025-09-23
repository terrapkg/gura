package repomd

import "encoding/xml"

// Represent the root structure of repomd.xml metadata for an RPM repository
// It contains information about available metadata files and their checksums.
type RPMRepomd struct {
	XMLName  xml.Name `xml:"repomd"`
	Revision string   `xml:"revision"`
	Data     []struct {
		Type            string       `xml:"type,attr"`
		Checksum        RPMChecksum  `xml:"checksum"`
		OpenChecksum    *RPMChecksum `xml:"open-checksum,omitempty"`
		HeaderChecksum  *RPMChecksum `xml:"header-checksum,omitempty"`
		Timestamp       int64        `xml:"timestamp"`
		Size            int64        `xml:"size"`
		OpenSize        *int64       `xml:"open-size,omitempty"`
		HeaderSize      *int64       `xml:"header-size,omitempty"`
		DatabaseVersion *int         `xml:"database_version,omitempty"`
		Location        struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
	} `xml:"data"`
}

// Checksum value and its type for repository metadata
type RPMChecksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"chardata"`
}

// Single package entry in primary.xml metadata
//
// Includes basic package information, versioning, checksums, and additional metadata in the Format field.
type RPMPackageXML struct {
	Name    string `xml:"name"`
	Arch    string `xml:"arch"`
	Version struct {
		Epoch string `xml:"epoch,attr"`
		Ver   string `xml:"ver,attr"`
		Rel   string `xml:"rel,attr"`
	} `xml:"version"`
	Checksum RPMChecksum `xml:"checksum"`
	Packager string      `xml:"packager"`
	Url      string      `xml:"url"`
	// time
	// size
	// location
	Format RPMFormat `xml:"format"`
}

// Additional metadata for an RPM package
type RPMFormat struct {
	License     *string    `xml:"license,omitempty"`
	Vendor      *string    `xml:"vendor,omitempty"`
	Group       *string    `xml:"group,omitempty"`
	Buildhost   *string    `xml:"buildhost,omitempty"`
	Sourcerpm   *string    `xml:"sourcerpm,omitempty"`
	Provides    []RPMEntry `xml:"provides>entry,omitempty"`
	Requires    []RPMEntry `xml:"requires>entry,omitempty"`
	Obsoletes   []RPMEntry `xml:"obsoletes>entry,omitempty"`
	Conflicts   []RPMEntry `xml:"conflicts>entry,omitempty"`
	Enhances    []RPMEntry `xml:"enhances>entry,omitempty"`
	Suggests    []RPMEntry `xml:"suggests>entry,omitempty"`
	Recommends  []RPMEntry `xml:"recommends>entry,omitempty"`
	Supplements []RPMEntry `xml:"supplements>entry,omitempty"`
}

// Single dependency or capability entry in RPM metadata
type RPMEntry struct {
	Name  string `xml:"name,attr"`
	Flags string `xml:"flags,attr"`
	Epoch string `xml:"epoch,attr"`
	Ver   string `xml:"ver,attr"`
	Rel   string `xml:"rel,attr"`
}

// Root structure of primary.xml metadata
//
// Contains a list of all packages available in the repository.
type RPMPrimaryXML struct {
	Packages []RPMPackageXML `xml:"package"`
}

// Additional metadata for an RPM package
type RPMMeta struct {
	Checksum RPMChecksum
	Packager string
	Url      string
	Format   RPMFormat
}
