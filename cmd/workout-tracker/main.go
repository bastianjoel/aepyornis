package main

import (
	"os"

	appassets "github.com/AepyornisNet/aepyornis/assets"
	"github.com/AepyornisNet/aepyornis/pkg/app"
	"github.com/AepyornisNet/aepyornis/pkg/version"
	apptranslations "github.com/AepyornisNet/aepyornis/translations"
)

var (
	gitRef     = "0.0.0-dev"
	gitRefName = "local"
	gitRefType = "local"
	gitCommit  = "local"
	buildTime  = "now"
)

func main() {
	a := app.NewApp(version.Version{
		BuildTime: buildTime,
		Ref:       gitRef,
		RefName:   gitRefName,
		RefType:   gitRefType,
		Sha:       gitCommit,
	})
	a.Assets = appassets.FS()
	a.Translations = apptranslations.FS()

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "development" {
		a.AssetDir = "assets"
	}

	if err := a.Configure(); err != nil {
		panic(err)
	}

	if err := a.Serve(); err != nil {
		panic(err)
	}
}
