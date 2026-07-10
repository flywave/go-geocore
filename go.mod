module github.com/flywave/go-geocore

go 1.25

require (
	github.com/flywave/go-gocad v0.0.0
	github.com/flywave/go-las v0.0.0
	github.com/flywave/go-omf v0.0.0
	github.com/flywave/go-segy v0.0.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/flywave/hdf5 v0.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/flywave/go-las => ../go-las

replace github.com/flywave/go-segy => ../go-segy

replace github.com/flywave/go-gocad => ../go-gocad

replace github.com/flywave/go-omf => ../go-omf

replace github.com/flywave/go-geology => ../go-geology

replace github.com/flywave/hdf5 => ../hdf5
