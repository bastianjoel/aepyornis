package main

import (
	appassets "github.com/AepyornisNet/aepyornis/assets"
	"github.com/AepyornisNet/aepyornis/pkg/app"
	"github.com/AepyornisNet/aepyornis/pkg/version"
	apptranslations "github.com/AepyornisNet/aepyornis/translations"
	"gorm.io/gorm"
)

type cli struct {
	app *app.App
}

func newCLI() (*cli, error) {
	a := app.NewApp(version.Version{
		BuildTime: buildTime,
		Ref:       gitRef,
		RefName:   gitRefName,
		RefType:   gitRefType,
		Sha:       gitCommit,
	})
	a.Assets = appassets.FS()
	a.Translations = apptranslations.FS()

	if err := a.ResetConfiguration(); err != nil {
		return nil, err
	}

	a.ConfigureLogger()

	if err := a.ConfigureDatabase(); err != nil {
		return nil, err
	}

	a.ConfigureGeocoder()

	c := &cli{
		app: a,
	}

	return c, nil
}

func (c *cli) getDatabase() *gorm.DB {
	return c.app.DB()
}
