package models

type ApplicationsSearchRequest struct {
	Names []string

	IncludeFields ApplicationSearchIncludeFields
}

type ApplicationSearchIncludeFields struct {
	LatestJobSummary bool
	Environments     bool
}
