package examples

import (
	"sort"

	"github.com/0funct0ry/squad/internal/examples/airline"
	"github.com/0funct0ry/squad/internal/examples/audio"
	"github.com/0funct0ry/squad/internal/examples/chat"
	"github.com/0funct0ry/squad/internal/examples/chess"
	"github.com/0funct0ry/squad/internal/examples/ecommerce"
	"github.com/0funct0ry/squad/internal/examples/elearning"
	"github.com/0funct0ry/squad/internal/examples/food_delivery"
	"github.com/0funct0ry/squad/internal/examples/hospital"
	"github.com/0funct0ry/squad/internal/examples/inventory"
	"github.com/0funct0ry/squad/internal/examples/library"
	"github.com/0funct0ry/squad/internal/examples/media"
	"github.com/0funct0ry/squad/internal/examples/network"
	"github.com/0funct0ry/squad/internal/examples/social_graph"
	"github.com/0funct0ry/squad/internal/examples/stay"
	"github.com/0funct0ry/squad/internal/examples/stocks"
	"github.com/0funct0ry/squad/internal/examples/streaming"
	"github.com/0funct0ry/squad/internal/examples/transit"
	"github.com/0funct0ry/squad/internal/examples/vcs"
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
		Description: "Minimal library management system schema",
		Schema:      library.Schema,
	},
	{
		Slug:        "stocks",
		Name:        "Stock Trading Platform",
		Description: "Minimal stock trading platform schema",
		Schema:      stocks.Schema,
	},
	{
		Slug:        "elearning",
		Name:        "E-Learning Platform",
		Description: "Minimal e-learning platform schema",
		Schema:      elearning.Schema,
	},
	{
		Slug:        "food-delivery",
		Name:        "Restaurant/Food Delivery",
		Description: "Minimal food delivery platform schema",
		Schema:      food_delivery.Scheme,
	},
	{
		Slug:        "inventory",
		Name:        "Supply Chain/Inventory Management",
		Description: "Minimal supply chain / inventory management system schema",
		Schema:      inventory.Schema,
	},
	{
		Slug:        "social-graph",
		Name:        "Social Network Graph",
		Description: "Minimal social network graph schema",
		Schema:      social_graph.Schema,
	},
	{
		Slug:        "chat",
		Name:        "Chat platform",
		Description: "Minimal chat platform schema",
		Schema:      chat.Schema,
	},
	{
		Slug:        "vcs",
		Name:        "Version Control System",
		Description: "Minimal version control system schema",
		Schema:      vcs.Schema,
	},
	{
		Slug:        "chess",
		Name:        "Online Chess Platform",
		Description: "Minimal online chess platform schema",
		Schema:      chess.Schema,
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
