//go:generate file2byteslice -package=images -input=./images/enemy.png -output=./images/enemy.go -var=Enemy_png
//go:generate file2byteslice -package=images -input=./images/player.png -output=./images/player.go -var=Player_png
//go:generate file2byteslice -package=images -input=./images/bullet.png -output=./images/bullet.go -var=Bullet_png
//go:generate gofmt -s -w .

package main

import (
	// Dummy imports for go.mod for some Go files with 'ignore' tags. For example, `go mod tidy` does not
	// recognize Go files with 'ignore' build tag.
	//
	// Note that this affects only importing this package, but not 'file2byteslice' commands in //go:generate.
	_ "github.com/hajimehoshi/file2byteslice"
)
