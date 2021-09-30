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
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	raudio "github.com/hajimehoshi/ebiten/v2/examples/resources/audio"

	resources "github.com/hajimehoshi/ebiten/v2/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/mariuseis/go-inn/images"
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

	maxMoveVelocity     = 3
	moveAcceleration    = 1
	gravityAcceleration = 1
	maxGravityVelocity  = 8
	jumpVelocity        = 8
)

var (
	gopherImage     *ebiten.Image
	enemyImage      *ebiten.Image
	tilesImage      *ebiten.Image
	titleArcadeFont font.Face
	arcadeFont      font.Face
	smallArcadeFont font.Face
)

//asset image declarations
func init() {
	// 1. create const "img" and use Gopher_png from resources
	img, _, err := image.Decode(bytes.NewReader(images.Player_png))
	// 2. handle image error
	if err != nil {
		log.Fatal(err)
	}
	// 3. declare the gopherImage and use the "img" defined above
	gopherImage = ebiten.NewImageFromImage(img)

	// All 3 main steps are repeated for other images, in this case -> floor tiles
	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(images.Enemy_png))
	if err != nil {
		log.Fatal(err)
	}
	enemyImage = ebiten.NewImageFromImage(img)
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

type BaseCollider struct {
	x int
	y int
}

type Collidable struct {
	baseCollider BaseCollider
	width        int
	height       int
}

type Platform struct {
	baseCollider BaseCollider
	tileCount    int
}

type Enemy struct {
	baseCollider BaseCollider
	vx           int
}

type Projectile struct {
	lifespan     int
	baseCollider BaseCollider
}

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

	enemies     []Enemy
	projectiles []Projectile

	gameoverCount int
	jumpCount     int

	touchIDs   []ebiten.TouchID
	gamepadIDs []ebiten.GamepadID

	audioContext *audio.Context
	jumpPlayer   *audio.Player
	hitPlayer    *audio.Player

	platforms []Platform
}

func NewGame() *Game {
	g := &Game{}
	g.init()
	return g
}

func (g *Game) init() {
	g.x16 = 0
	g.y16 = 100
	g.cameraX = -240
	g.cameraY = 0
	g.pipeTileYs = make([]int, 256)
	for i := range g.pipeTileYs {
		g.pipeTileYs[i] = rand.Intn(6) + 2
	}
	g.jumpCount = 0

	enemyA := Enemy{baseCollider: BaseCollider{x: 320, y: 400}, vx: 2}
	enemyB := Enemy{baseCollider: BaseCollider{x: 0, y: 150}, vx: 2}
	g.enemies = []Enemy{enemyA, enemyB}

	if g.audioContext == nil {
		g.audioContext = audio.NewContext(48000)
	}

	jumpD, err := vorbis.Decode(g.audioContext, bytes.NewReader(raudio.Jump_ogg))
	fmt.Println(jumpD, err)
	if err != nil {
		log.Fatal(err)
	}
	g.jumpPlayer, err = audio.NewPlayer(g.audioContext, jumpD)
	if err != nil {
		log.Fatal(err)
	}

	jabD, err := wav.Decode(g.audioContext, bytes.NewReader(raudio.Jab_wav))
	if err != nil {
		log.Fatal(err)
	}
	g.hitPlayer, err = audio.NewPlayer(g.audioContext, jabD)
	if err != nil {
		log.Fatal(err)
	}
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

	if g.isKeyJustPressed() {
		// not more than 2 jumps
		// allow jump from collision/platforms
		if g.jumpCount < 2 {
			g.vy16 = -jumpVelocity * 2
			g.jumpCount++
		} else if g.hit() || g.groundTouch() {
			g.jumpCount = 0
		}
		g.jumpPlayer.Rewind()
		g.jumpPlayer.Play()
	}

	// if !g.hit() {
	if areBothPressed {
		g.vx16 = 0
	} else if isLeftPressed {
		g.vx16 -= moveAcceleration
		if g.vx16 < -maxMoveVelocity {
			g.vx16 = -maxMoveVelocity
		}
		g.cameraX += g.vx16
	} else if isRightPressed {
		g.vx16 += moveAcceleration
		if g.vx16 > maxMoveVelocity {
			g.vx16 = maxMoveVelocity
		}
		g.cameraX += g.vx16
	} else {
		g.vx16 = 0
	}

	g.x16 += g.vx16
	g.y16 += g.vy16
	// } else {
	// 	g.vx16 = 0
	// 	g.vy16 = 0
	// }
}

func (g *Game) isKeyPressed(keys []ebiten.Key) bool {
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
		if v == -1 {
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
		if g.isRestartJustPressed() {
			g.mode = ModeGameOver
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyF) {
			g.projectiles = append(g.projectiles, Projectile{baseCollider: BaseCollider{x: (g.x16 + screenWidth/2), y: screenHeight - 60 - (6160 - g.y16)}, lifespan: 100})
		}

		g.handleMovement()
		g.moveEnemies()

		// Gravity
		g.vy16 += gravityAcceleration
		if g.vy16 > maxGravityVelocity {
			g.vy16 = maxGravityVelocity
		}

		// if g.hit() {
		// 	// fmt.Printf("it is hit")
		// 	// g.hitPlayer.Rewind()
		// 	// g.hitPlayer.Play()
		// 	//g.mode = ModeGameOver
		// 	//g.gameoverCount = 30
		// 	//g.vy16 = 0
		// 	// g.vx16 = 0
		// 	// fmt.Print("-----COLISSION-----")
		// }

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
	for i := len(g.projectiles) - 1; i >= 0; i-- {
		g.drawProjectile(screen, g.projectiles[i])
		g.projectiles[i].baseCollider.x += 3
		g.projectiles[i].lifespan -= 1
		if g.projectiles[i].lifespan < 1 {
			g.projectiles = append(g.projectiles[:i], g.projectiles[i+1:]...)
		}
	}
	platformA := Platform{baseCollider: BaseCollider{x: 400, y: 200}, tileCount: 10}
	platformB := Platform{baseCollider: BaseCollider{x: 320, y: 400}, tileCount: 4}
	g.platforms = []Platform{platformA, platformB}
	g.drawPlatforms(screen, g.platforms)
	if g.mode != ModeTitle {
		g.drawGopher(screen)
		g.drawEnemies(screen, g.enemies)
	}
	var titleTexts []string
	var texts []string
	switch g.mode {
	case ModeTitle:
		titleTexts = []string{"GO INN"}
		texts = []string{"", "", "", "", "", "", "", "PRESS SPACE KEY"}
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
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f X: %1d Y: %2d VX: %1d VY: %2d", ebiten.CurrentTPS(), g.x16, g.y16, g.vx16, g.vy16))
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
	x := g.x16 / tileSize
	if (x - pipeStartOffsetX) <= 0 {
		return 0
	}
	return floorDiv(x-pipeStartOffsetX, pipeIntervalX)
}

func (g *Game) hit() bool {

	const (
		gopherWidth  = 30
		gopherHeight = 60
	)

	// w, h := gopherImage.Size()

	// fmt.Printf("gopher stats HARDCODED w: %d, h: %d", gopherWidth, gopherHeight)
	// fmt.Printf("gopher stats w: %d, h: %d", w, h)

	// x1 := x0 + gopherWidth
	// y1 := y0 + gopherHeight

	// wTile, hTile := tilesImage.Size()
	// x1 := x0 + gopherWidth
	// y1 := y0 + gopherHeight

	for i := 0; i < len(g.platforms); i++ {
		p := g.platforms[i]
		// Platform{baseCollider: BaseCollider{x: 400, y: 200}, tileCount: 10}
		player := Collidable{baseCollider: BaseCollider{x: g.x16, y: g.y16}, width: gopherWidth, height: gopherHeight}
		platform := Collidable{baseCollider: BaseCollider{x: p.baseCollider.x, y: p.baseCollider.y}, width: p.tileCount * tileSize, height: tileSize}

		// y0 := floorDiv(g.y16, 16) + (h-gopherHeight)/2

		// fmt.Printf("+++++ player x: %d, y: %d, width: %d, height: %d", player.x, player.y, player.width, player.height)
		// fmt.Printf("----- platform x: %d, y: %d, width: %d, height: %d", platform.x, platform.y, platform.width, platform.height)

		if player.baseCollider.x < (platform.baseCollider.x+platform.width) &&
			(player.baseCollider.x+player.width) > platform.baseCollider.x &&
			player.baseCollider.y < (platform.baseCollider.y+platform.height) &&
			(player.baseCollider.y+player.height) > platform.baseCollider.y {
			// fmt.Printf("IT COLLIDES")
			return true
		}
	}

	return false
}

func (g *Game) drawPlatforms(screen *ebiten.Image, platforms []Platform) {
	op := &ebiten.DrawImageOptions{}

	for _, platform := range platforms {
		for i := 0; i < platform.tileCount; i++ {
			op.GeoM.Reset()
			op.GeoM.Translate(float64(platform.baseCollider.x+tileSize*i-g.cameraX), float64(platform.baseCollider.y))
			screen.DrawImage(tilesImage.SubImage(image.Rect(0, 290, tileSize, 290+tileSize)).(*ebiten.Image), op)
		}
	}
}

func (g *Game) drawProjectile(screen *ebiten.Image, projectile Projectile) {
	op := &ebiten.DrawImageOptions{}

	op.GeoM.Reset()
	op.GeoM.Translate(float64(projectile.baseCollider.x+(g.cameraX-g.x16)/15), float64(projectile.baseCollider.y))
	screen.DrawImage(tilesImage.SubImage(image.Rect(0, 290, tileSize, 290+tileSize)).(*ebiten.Image), op)
}

func (g *Game) groundTouch() bool {
	const gopherHeight = 60
	_, h := gopherImage.Size()

	y0 := g.y16 + (h-gopherHeight)/2
	y1 := y0 + gopherHeight

	if y1 >= screenHeight-tileSize {
		// fmt.Printf("---ground---")
		return true
	}
	return false
}

func (g *Game) moveEnemies() {
	// enemy.vx -= moveAcceleration
	// if enemy.vx < -2 {
	// 	enemy.vx = -2
	// }
	for index := range g.enemies {
		g.enemies[index].baseCollider.x -= 1
	}

	// g.cameraX += g.vx16

	// call on update
	// reduce value of enemy's x position by 1
	//will it repaint enemy on gopherDraw?
}

func flipAsset(image *ebiten.Image, op *ebiten.DrawImageOptions) {
	w, _ := enemyImage.Size()

	op.GeoM.Scale(-1, 1)
	op.GeoM.Translate(float64(w), 0)
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
	op.GeoM.Translate(float64(g.x16)-float64(g.cameraX), float64(g.y16)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(gopherImage, op)
}

func (g *Game) drawEnemies(screen *ebiten.Image, enemies []Enemy) {
	op := &ebiten.DrawImageOptions{}
	// w, h := enemyImage.Size()

	for _, enemy := range enemies {
		op.GeoM.Reset()
		//flip asset
		flipAsset(enemyImage, op)

		// //place at right bottom, behind the initial screen
		// op.GeoM.Translate(float64(screenWidth/2 + w * 2), float64(screenHeight - h))

		// // make it sit on terain, idk about the division by 3, just works
		// op.GeoM.Translate(0, -float64(h)/3.0)
		fmt.Println(enemy.baseCollider.x)

		op.GeoM.Translate(float64(enemy.baseCollider.x-g.cameraX), float64(enemy.baseCollider.y))
		op.Filter = ebiten.FilterLinear
		screen.DrawImage(enemyImage, op)
	}

	//make check for position to approach each other
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Go Inn")
	if err := ebiten.RunGame(NewGame()); err != nil {
		panic(err)
	}
}
