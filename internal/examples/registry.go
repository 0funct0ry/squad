package examples

import (
	"sort"

	"github.com/0funct0ry/squad/internal/examples/airline"
	"github.com/0funct0ry/squad/internal/examples/audio"
	"github.com/0funct0ry/squad/internal/examples/ecommerce"
	"github.com/0funct0ry/squad/internal/examples/hospital"
	"github.com/0funct0ry/squad/internal/examples/library"
	"github.com/0funct0ry/squad/internal/examples/media"
	"github.com/0funct0ry/squad/internal/examples/network"
	"github.com/0funct0ry/squad/internal/examples/stay"
	"github.com/0funct0ry/squad/internal/examples/streaming"
	"github.com/0funct0ry/squad/internal/examples/transit"
	"github.com/0funct0ry/squad/internal/examples/video"
)

// registry is the manually maintained list of canned example data models.
// Adding a new one is: create a subpackage with a Schema constant, then add
// one line here. No other code changes are required.
var registry = []Example{
	{
		Slug:        "network",
		Name:        "Professional Networking Platform",
		Description: "Minimal professional networking schema",
		Schema:      network.Schema,
	},
	{
		Slug:        "video",
		Name:        "Video Sharing Platform",
		Description: "Minimal video sharing schema",
		Schema:      video.Schema,
	},
	{
		Slug:        "media",
		Name:        "Visual Social Network",
		Description: "Minimal visual social network schema",
		Schema:      media.Schema,
	},
	{
		Slug:        "transit",
		Name:        "Ride-Hailing Service",
		Description: "Minimal ride-hailing service schema",
		Schema:      transit.Schema,
	},
	{
		Slug:        "audio",
		Name:        "Audio Streaming Service",
		Description: "Minimal audio streaming service schema",
		Schema:      audio.Schema,
	},
	{
		Slug:        "stay",
		Name:        "Peer-to-Peer Lodging Marketplace",
		Description: "Minimal peer-to-peer lodging marketplace schema",
		Schema:      stay.Schema,
	},
	{
		Slug:        "ecommerce",
		Name:        "E-Commerce Platform",
		Description: "Minimal E-Commerce platform schema",
		Schema:      ecommerce.Schema,
	},
	{
		Slug:        "hospital",
		Name:        "Hospital/EHR System",
		Description: "Minimal Hospital/Electronic Health Record system schema",
		Schema:      hospital.Schema,
	},
	{
		Slug:        "airline",
		Name:        "Airline Reservation System",
		Description: "Minimal Airline reservation system schema",
		Schema:      airline.Schema,
	},
	{
		Slug:        "streaming",
		Name:        "Movie Streaming Platform",
		Description: "Minimal Movie streaming platform schema",
		Schema:      streaming.Schema,
	},
	{
		Slug:        "library",
		Name:        "Library Management System",
		Description: "Minimal Library management system schema",
		Schema:      library.Schema,
	},
}

// All returns every registered example, sorted by Name.
func All() []Example {
	out := make([]Example, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ByName returns the example with the given slug, if any.
func ByName(slug string) (Example, bool) {
	for _, e := range registry {
		if e.Slug == slug {
			return e, true
		}
	}
	return Example{}, false
}
