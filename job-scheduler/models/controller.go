package models

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
}
