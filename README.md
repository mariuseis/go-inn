# go-inn

write this into console:

`go run -tags=example github.com/hajimehoshi/ebiten/v2/examples/rotate`

run main.go with:

`go run main.go` MAIN GO IS FLAPPY GOPHER

# Generating images

* Run command
`go install github.com/hajimehoshi/file2byteslice/cmd/file2byteslice@latest`

* Add new .png to images folder. For example - `enemy.png`

* Update `generate-assets.go` by existing example. For example:
`//go:generate file2byteslice -package=images -input=./images/enemy.png -output=./images/enemy.go -var=Enemy_png`

* Run `go generate`

* Use images.Enemy_png in code

