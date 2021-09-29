// Copyright 2018 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build example
// +build example

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	resources "github.com/hajimehoshi/ebiten/v2/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func floorDiv(x, y int) int {
	d := x / y
	if d*y == x || x >= 0 {
		return d
	}
	return d - 1
}

func floorMod(x, y int) int {
	return x - floorDiv(x, y)*y
}

const (
	screenWidth      = 640
	screenHeight     = 480
	tileSize         = 32
	titleFontSize    = fontSize * 1.5
	fontSize         = 24
	smallFontSize    = fontSize / 2
	pipeWidth        = tileSize * 2
	pipeStartOffsetX = -1
	pipeIntervalX    = 8
	pipeGapY         = 5
)

var (
	gopherImage     *ebiten.Image
	enemyImage		*ebiten.Image
	tilesImage      *ebiten.Image
	titleArcadeFont font.Face
	arcadeFont      font.Face
	smallArcadeFont font.Face
)

//asset image declarations
func init() {
	// 1. create const "img" and use Gopher_png from resources
	img, _, err := image.Decode(bytes.NewReader(resources.Gopher_png))
	// 2. handle image error
	if err != nil {
		log.Fatal(err)
	}
	// 3. declare the gopherImage and use the "img" defined above
	gopherImage = ebiten.NewImageFromImage(img)
	// All 3 main steps are repeated for other images, in this case -> floor tiles

	// TODO add enemy asset, read enemy asset image
	// img, _, err := image.Decode(bytes.NewReader(resources.TODO_ENEMY_png))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	enemyImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage = ebiten.NewImageFromImage(img)
}
//text font declarations
func init() {
	tt, err := opentype.Parse(fonts.PressStart2P_ttf)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	titleArcadeFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    titleFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	arcadeFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	smallArcadeFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    smallFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
}

type Mode int

const (
	ModeTitle Mode = iota
	ModeGame
	ModeGameOver
)

type Game struct {
	mode Mode

	// The gopher's position
	x16  int
	y16  int
	vy16 int
	vx16 int

	// Camera
	cameraX int
	cameraY int

	// Pipes
	pipeTileYs []int

	gameoverCount int

	touchIDs   []ebiten.TouchID
	gamepadIDs []ebiten.GamepadID

	audioContext *audio.Context
	jumpPlayer   *audio.Player
	hitPlayer    *audio.Player
}

func NewGame() *Game {
	g := &Game{}
	g.init()
	return g
}

func (g *Game) init() {
	g.x16 = 0
	g.y16 = 100 * 16
	g.cameraX = -240
	g.cameraY = 0
	g.pipeTileYs = make([]int, 256)
	for i := range g.pipeTileYs {
		g.pipeTileYs[i] = rand.Intn(6) + 2
	}

	// if g.audioContext == nil {
	// 	g.audioContext = audio.NewContext(48000)
	// }

	// jumpD, err := vorbis.Decode(g.audioContext, bytes.NewReader(raudio.Jump_ogg))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// g.jumpPlayer, err = g.audioContext.NewPlayer(jumpD)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// jabD, err := wav.Decode(g.audioContext, bytes.NewReader(raudio.Jab_wav))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// g.hitPlayer, err = g.audioContext.NewPlayer(jabD)
	// if err != nil {
	// 	log.Fatal(err)
	// }
}

func (g *Game) isKeyJustPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	return false
}

func (g *Game) handleMovement() {
	isLeftPressed := g.isKeyPressed([]ebiten.Key{ebiten.KeyA}) || g.isKeyPressed([]ebiten.Key{ebiten.KeyArrowLeft})
	isRightPressed := g.isKeyPressed([]ebiten.Key{ebiten.KeyD}) || g.isKeyPressed([]ebiten.Key{ebiten.KeyArrowRight})
	areBothPressed := g.isKeyPressed([]ebiten.Key{ebiten.KeyA, ebiten.KeyD}) || g.isKeyPressed([]ebiten.Key{ebiten.KeyArrowLeft, ebiten.KeyArrowRight})

	if (areBothPressed) {
		g.vx16 = 0;
	} else if (isLeftPressed) {
		g.vx16 -= 10
		g.cameraX -= 2
		if(g.vx16 < -32) {
			g.vx16 = -32
		}
	} else if (isRightPressed) {
		g.vx16 += 10
		g.cameraX += 2
		if(g.vx16 > 32) {
			g.vx16 = 32
		}
	} else {
		g.vx16 = 0;
	}

	g.x16 += g.vx16
	g.y16 += g.vy16
}

func (g *Game) isKeyPressed(keys []ebiten.Key) bool{
	keyMap := make(map[ebiten.Key]int)
	for _, key := range keys {
		keyMap[key] = -1
	}
	for _, v := range inpututil.PressedKeys() {
		if keyMap[v] == -1 {
			keyMap[v] = 1
		}
	}
	for _, v := range keyMap {
		if (v == -1) {
			return false
		} 
	}
	return true
}

func (g *Game) isRestartJustPressed() bool {
	return g.isKeyPressed([]ebiten.Key{ebiten.KeyControlLeft, ebiten.KeyR})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) Update() error {
	switch g.mode {
	case ModeTitle:
		if g.isKeyJustPressed() {
			g.mode = ModeGame
		}
	case ModeGame:
		//g.x16 += 32
		//g.cameraX += 2
		if g.isKeyJustPressed() {
			if(g.y16 > 5000) { // if y position of Gopher is higher than 5000 then stop changing position (initial is 6100)
				g.vy16 = -80 // on jump change position by -80
			}
			// g.jumpPlayer.Rewind()
			// g.jumpPlayer.Play()
		}

		if g.isRestartJustPressed(){
			g.mode = ModeGameOver
		}

		g.handleMovement()

		// Gravity
		g.vy16 += 4
		if g.vy16 > 96 {
			g.vy16 = 96
		}

		if g.hit() {
			// fmt.Printf("it is hit")
			// g.hitPlayer.Rewind()
			// g.hitPlayer.Play()
			//g.mode = ModeGameOver
			//g.gameoverCount = 30
			//g.vy16 = 0
			// g.vx16 = 0
			// fmt.Print("-----PIPE-----")
		}

		if g.groundTouch() {
			g.vy16 = 0
		}
	case ModeGameOver:
		if g.gameoverCount > 0 {
			g.gameoverCount--
		}
		if g.gameoverCount == 0 && g.isKeyJustPressed() {
			g.init()
			g.mode = ModeTitle
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x80, 0xa0, 0xc0, 0xff}) //background color
	g.drawTiles(screen)
	if g.mode != ModeTitle {
		g.drawGopher(screen)
		g.drawEnemy(screen)
	}
	var titleTexts []string
	var texts []string
	switch g.mode {
	case ModeTitle:
		titleTexts = []string{"FLAPPY GOPHER"}
		texts = []string{"", "", "", "", "", "", "", "PRESS SPACE KEY", "", "OR A/B BUTTON", "", "OR TOUCH SCREEN"}
	case ModeGameOver:
		texts = []string{"", "GAME OVER!"}
	}
	for i, l := range titleTexts {
		x := (screenWidth - len(l)*titleFontSize) / 2
		text.Draw(screen, l, titleArcadeFont, x, (i+4)*titleFontSize, color.White)
	}
	for i, l := range texts {
		x := (screenWidth - len(l)*fontSize) / 2
		text.Draw(screen, l, arcadeFont, x, (i+4)*fontSize, color.White)
	}

	if g.mode == ModeTitle {
		msg := []string{
			"Go Gopher by Renee French is",
			"licenced under CC BY 3.0.",
		}
		for i, l := range msg {
			x := (screenWidth - len(l)*smallFontSize) / 2
			text.Draw(screen, l, smallArcadeFont, x, screenHeight-4+(i-1)*smallFontSize, color.White)
		}
	}

	scoreStr := fmt.Sprintf("%04d", g.score())
	text.Draw(screen, scoreStr, arcadeFont, screenWidth-len(scoreStr)*fontSize, fontSize, color.White)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS()))
}

func (g *Game) pipeAt(tileX int) (tileY int, ok bool) {
	if (tileX - pipeStartOffsetX) <= 0 {
		return 0, false
	}
	if floorMod(tileX-pipeStartOffsetX, pipeIntervalX) != 0 {
		return 0, false
	}
	idx := floorDiv(tileX-pipeStartOffsetX, pipeIntervalX)
	return g.pipeTileYs[idx%len(g.pipeTileYs)], true
}

func (g *Game) score() int {
	x := floorDiv(g.x16, 16) / tileSize
	if (x - pipeStartOffsetX) <= 0 {
		return 0
	}
	return floorDiv(x-pipeStartOffsetX, pipeIntervalX)
}

func (g *Game) hit() bool {
	if g.mode != ModeGame {
		return false
	}
	const (
		gopherWidth  = 30
		gopherHeight = 60
	)
	w, h := gopherImage.Size()
	x0 := floorDiv(g.x16, 16) + (w-gopherWidth)/2
	y0 := floorDiv(g.y16, 16) + (h-gopherHeight)/2
	x1 := x0 + gopherWidth
	y1 := y0 + gopherHeight
	if y0 < -tileSize*4 {
		return true
	}
	xMin := floorDiv(x0-pipeWidth, tileSize)
	xMax := floorDiv(x0+gopherWidth, tileSize)
	for x := xMin; x <= xMax; x++ {
		y, ok := g.pipeAt(x)
		if !ok {
			continue
		}
		if x0 >= x*tileSize+pipeWidth {
			continue
		}
		if x1 < x*tileSize {
			continue
		}
		if y0 < y*tileSize {
			return true
		}
		if y1 >= (y+pipeGapY)*tileSize {
			return true
		}
	}
	return false
}

func (g *Game) groundTouch() bool {
	const gopherHeight = 60
	_, h := gopherImage.Size()

	y0 := floorDiv(g.y16, 16) + (h-gopherHeight)/2
	y1 := y0 + gopherHeight

	if y1 >= screenHeight-tileSize {
		// fmt.Printf("---ground---")
		return true
	}

	return false
}

func (g *Game) drawTiles(screen *ebiten.Image) {
	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	op := &ebiten.DrawImageOptions{}
	for i := -2; i < nx+1; i++ {
		// ground
		op.GeoM.Reset()
		op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
			float64((ny-1)*tileSize-floorMod(g.cameraY, tileSize)))
		screen.DrawImage(tilesImage.SubImage(image.Rect(0, 0, tileSize, tileSize)).(*ebiten.Image), op)

		// pipe
		if tileY, ok := g.pipeAt(floorDiv(g.cameraX, tileSize) + i); ok {
			for j := 0; j < tileY; j++ {
				op.GeoM.Reset()
				op.GeoM.Scale(1, -1)
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				op.GeoM.Translate(0, tileSize)
				var r image.Rectangle
				if j == tileY-1 {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize*2)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
			for j := tileY + pipeGapY; j < screenHeight/tileSize-1; j++ {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				var r image.Rectangle
				if j == tileY+pipeGapY {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize+tileSize)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
		}
	}
}

func (g *Game) drawGopher(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	w, h := gopherImage.Size()
	op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
	op.GeoM.Rotate(float64(g.vy16) / 96.0 * math.Pi / 6)
	op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	op.GeoM.Translate(float64(g.x16/16.0)-float64(g.cameraX), float64(g.y16/16.0)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(gopherImage, op)
}

func (g *Game) drawEnemy(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	w, h := enemyImage.Size()

	x := float64(screenWidth - w)
	y := float64(screenHeight - h - 20) // draw above ground at the front

	// fmt.Printf("www = %g, hhh = %g \n", x, y)
	op.GeoM.Translate(x, y)
	// op.GeoM.Rotate(float64(g.vy16) / 96.0 * math.Pi / 6)
	// op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	// op.GeoM.Translate(float64(g.x16/16.0)-float64(g.cameraX), float64(g.y16/16.0)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(enemyImage, op)
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Go Inn")
	if err := ebiten.RunGame(NewGame()); err != nil {
		panic(err)
	}
}