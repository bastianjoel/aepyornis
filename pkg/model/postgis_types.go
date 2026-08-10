package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// PostGISPoint represents a PostGIS Point geometry column for GORM AutoMigrate.
type PostGISPoint string

// GormDBDataType returns the column data type for GORM AutoMigrate based on database dialect.
func (PostGISPoint) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "geometry(Point, 4326)"
	case "mysql":
		return "geometry"
	default:
		return "text"
	}
}

// PostGISGeometry represents a PostGIS Geometry column for GORM AutoMigrate.
type PostGISGeometry string

// GormDBDataType returns the column data type for GORM AutoMigrate based on database dialect.
func (PostGISGeometry) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "geometry(Geometry, 4326)"
	case "mysql":
		return "geometry"
	default:
		return "text"
	}
}
