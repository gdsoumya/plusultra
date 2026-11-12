// Command plusultra is the self-hosted server.
package main

import (
	"errors"
	"fmt"
	"os"
	_ "time/tzdata" // users pick IANA zones; slim images ship no zoneinfo

	"github.com/gdsoumya/plusultra/internal/version"
)

const usage = "usage: plusultra serve [flags] | migrate | seed-catalog | invite [flags] | version | " +
	"import-exercises <free-exercise-db dir> [images dir] | import-foods usda|indb|ifct|concepts [path]"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string) error {
	switch {
	case len(args) >= 1 && args[0] == "serve":
		return serve(args[1:])
	case len(args) == 1 && args[0] == "migrate":
		return migrate()
	case len(args) == 1 && args[0] == "seed-catalog":
		return seedCatalog()
	case len(args) >= 1 && args[0] == "invite":
		return invite(args[1:])
	case len(args) == 1 && args[0] == "version":
		fmt.Println(version.Version)
	case len(args) >= 2 && len(args) <= 3 && args[0] == "import-exercises":
		return importExercises(args[1:])
	case len(args) >= 2 && len(args) <= 3 && args[0] == "import-foods":
		return importFoods(args[1:])
	default:
		return errors.New(usage)
	}
	return nil
}
