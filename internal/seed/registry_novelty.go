package seed

import "github.com/brianvoe/gofakeit/v7"

// categoryOption builds the standard "category" option schema for a novelty
// generator, defaulting to the first choice.
func categoryOption(choices []string) []OptionField {
	def := ""
	if len(choices) > 0 {
		def = choices[0]
	}
	return []OptionField{
		{Key: "category", Label: "Category", Kind: OptKindSelect, Default: def, Choices: choices},
	}
}

func noveltyGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "airline", Group: "novelty", Description: "Airline theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"aircraftType"}), Fn: func(_ string, opts map[string]any) (any, error) {
			return gofakeit.AirlineAircraftType(), nil
		}},
		{Name: "animal", Group: "novelty", Description: "Animal theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"animal", "type"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "animal") {
			case "type":
				return gofakeit.AnimalType(), nil
			default:
				return gofakeit.Animal(), nil
			}
		}},
		{Name: "app", Group: "novelty", Description: "App theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"name", "version", "author"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "name") {
			case "version":
				return gofakeit.AppVersion(), nil
			case "author":
				return gofakeit.AppAuthor(), nil
			default:
				return gofakeit.AppName(), nil
			}
		}},
		{Name: "beer", Group: "novelty", Description: "Beer theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"name", "style", "hop", "yeast", "malt", "alcohol"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "name") {
			case "style":
				return gofakeit.BeerStyle(), nil
			case "hop":
				return gofakeit.BeerHop(), nil
			case "yeast":
				return gofakeit.BeerYeast(), nil
			case "malt":
				return gofakeit.BeerMalt(), nil
			case "alcohol":
				return gofakeit.BeerAlcohol(), nil
			default:
				return gofakeit.BeerName(), nil
			}
		}},
		{Name: "book", Group: "novelty", Description: "Book theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"title", "author", "genre"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "title") {
			case "author":
				return gofakeit.BookAuthor(), nil
			case "genre":
				return gofakeit.BookGenre(), nil
			default:
				return gofakeit.BookTitle(), nil
			}
		}},
		{Name: "car", Group: "novelty", Description: "Car theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"maker", "model", "type", "fuelType", "transmissionType", "modelYear", "vin"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "maker") {
			case "model":
				return gofakeit.CarModel(), nil
			case "type":
				return gofakeit.CarType(), nil
			case "fuelType":
				return gofakeit.CarFuelType(), nil
			case "transmissionType":
				return gofakeit.CarTransmissionType(), nil
			case "modelYear":
				return int64(gofakeit.Year()), nil
			case "vin":
				// hand-rolled 17-char VIN-shaped string (not check-digit validated)
				return gofakeit.Regex("[A-HJ-NPR-Z0-9]{17}"), nil
			default:
				return gofakeit.CarMaker(), nil
			}
		}},
		{Name: "celebrity", Group: "novelty", Description: "Celebrity theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"actor", "business", "sport"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "actor") {
			case "business":
				return gofakeit.CelebrityBusiness(), nil
			case "sport":
				return gofakeit.CelebritySport(), nil
			default:
				return gofakeit.CelebrityActor(), nil
			}
		}},
		{Name: "emoji", Group: "novelty", Description: "Emoji theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"emoji", "category", "alias", "tag"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "emoji") {
			case "category":
				return gofakeit.EmojiCategory(), nil
			case "alias":
				return gofakeit.EmojiAlias(), nil
			case "tag":
				return gofakeit.EmojiTag(), nil
			default:
				return gofakeit.Emoji(), nil
			}
		}},
		{Name: "error", Group: "novelty", Description: "Error message theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"generic", "database", "grpc", "http", "httpClient", "httpServer", "runtime"}), Fn: func(_ string, opts map[string]any) (any, error) {
			var err error
			switch optString(opts, "category", "generic") {
			case "database":
				err = gofakeit.ErrorDatabase()
			case "grpc":
				err = gofakeit.ErrorGRPC()
			case "http":
				err = gofakeit.ErrorHTTP()
			case "httpClient":
				err = gofakeit.ErrorHTTPClient()
			case "httpServer":
				err = gofakeit.ErrorHTTPServer()
			case "runtime":
				err = gofakeit.ErrorRuntime()
			default:
				err = gofakeit.Error()
			}
			if err == nil {
				return "", nil
			}
			return err.Error(), nil
		}},
		{Name: "game", Group: "novelty", Description: "Game theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"gamertag"}), Fn: func(_ string, opts map[string]any) (any, error) {
			return gofakeit.Gamertag(), nil
		}},
		{Name: "hacker", Group: "novelty", Description: "Hacker jargon theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"phrase", "abbreviation", "adjective", "noun", "verb"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "phrase") {
			case "abbreviation":
				return gofakeit.HackerAbbreviation(), nil
			case "adjective":
				return gofakeit.HackerAdjective(), nil
			case "noun":
				return gofakeit.HackerNoun(), nil
			case "verb":
				return gofakeit.HackerVerb(), nil
			default:
				return gofakeit.HackerPhrase(), nil
			}
		}},
		{Name: "hipster", Group: "novelty", Description: "Hipster jargon theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"word", "sentence", "paragraph"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "word") {
			case "sentence":
				return gofakeit.HipsterSentence(), nil
			case "paragraph":
				return gofakeit.HipsterParagraph(), nil
			default:
				return gofakeit.HipsterWord(), nil
			}
		}},
		{Name: "language", Group: "novelty", Description: "Language theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"name", "abbreviation"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "name") {
			case "abbreviation":
				return gofakeit.LanguageAbbreviation(), nil
			default:
				return gofakeit.Language(), nil
			}
		}},
		{Name: "minecraft", Group: "novelty", Description: "Minecraft theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{
			"ore", "wood", "armorTier", "armorPart", "weapon", "tool", "dye", "food",
			"animal", "villagerJob", "villagerStation", "villagerLevel",
			"mobPassive", "mobNeutral", "mobHostile", "mobBoss", "biome", "weather",
		}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "ore") {
			case "wood":
				return gofakeit.MinecraftWood(), nil
			case "armorTier":
				return gofakeit.MinecraftArmorTier(), nil
			case "armorPart":
				return gofakeit.MinecraftArmorPart(), nil
			case "weapon":
				return gofakeit.MinecraftWeapon(), nil
			case "tool":
				return gofakeit.MinecraftTool(), nil
			case "dye":
				return gofakeit.MinecraftDye(), nil
			case "food":
				return gofakeit.MinecraftFood(), nil
			case "animal":
				return gofakeit.MinecraftAnimal(), nil
			case "villagerJob":
				return gofakeit.MinecraftVillagerJob(), nil
			case "villagerStation":
				return gofakeit.MinecraftVillagerStation(), nil
			case "villagerLevel":
				return gofakeit.MinecraftVillagerLevel(), nil
			case "mobPassive":
				return gofakeit.MinecraftMobPassive(), nil
			case "mobNeutral":
				return gofakeit.MinecraftMobNeutral(), nil
			case "mobHostile":
				return gofakeit.MinecraftMobHostile(), nil
			case "mobBoss":
				return gofakeit.MinecraftMobBoss(), nil
			case "biome":
				return gofakeit.MinecraftBiome(), nil
			case "weather":
				return gofakeit.MinecraftWeather(), nil
			default:
				return gofakeit.MinecraftOre(), nil
			}
		}},
		{Name: "misc", Group: "novelty", Description: "Coin flip / weighted choice", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"flipACoin", "weightedChoice"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "flipACoin") {
			case "weightedChoice":
				choices := []string{"common", "common", "common", "uncommon", "uncommon", "rare"}
				return choices[gofakeit.Number(0, len(choices)-1)], nil
			default:
				return gofakeit.FlipACoin(), nil
			}
		}},
		{Name: "movie", Group: "novelty", Description: "Movie theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"name", "genre"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "name") {
			case "genre":
				return gofakeit.MovieGenre(), nil
			default:
				return gofakeit.MovieName(), nil
			}
		}},
		{Name: "school", Group: "novelty", Description: "School name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.School(), nil
		}},
		{Name: "song", Group: "novelty", Description: "Song theme pack", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"name", "artist", "genre"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "name") {
			case "artist":
				return gofakeit.SongArtist(), nil
			case "genre":
				return gofakeit.SongGenre(), nil
			default:
				return gofakeit.SongName(), nil
			}
		}},
	}
}
