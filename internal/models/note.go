// Package models defines the GORM models for the Sheidan demo app.
package models

import "gorm.io/gorm"

// Note stores a titled piece of text. It is the sample model for the
// demo app, and it shows the GORM model pattern for Sheidan apps:
// an embedded gorm.Model for the primary key and timestamps, plus
// the app's own fields.
type Note struct {
	gorm.Model
	Title string
	Body  string
}
