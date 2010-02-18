package main

import (
	"fmt"
	"os"
	"image"
	"time"
	"runtime"
	"rand"
	"exp/draw"
	"exp/draw/x11"
)

type Point struct {
	y		int
	x		int
}

type Block struct {
	rotate		int
	p			[3]Point
	color		image.RGBAColor
}

type Status struct {
	y		int
	x		int
	ty		int
	rotate	int
}

type Game struct {
	current		Status
	random		*rand.Rand
	img			draw.Image
	context		draw.Context
	board		[25][12]int
}

const (
	MASU_X		= 10
	MASU_Y		= 20
	BLOCK_PIXEL	= 24	
)

var block [8]Block = [8]Block{
	Block{ 1, [3]Point{Point{ 0, 0}, Point{ 0, 0}, Point{ 0, 0}}, image.RGBAColor{   0,   0,   0, 255}},	// null
	Block{ 2, [3]Point{Point{ 0,-1}, Point{ 0, 1}, Point{ 0, 2}}, image.RGBAColor{ 255,   0,   0, 255}},	// tetris
	Block{ 1, [3]Point{Point{ 1, 0}, Point{ 0, 1}, Point{ 1, 1}}, image.RGBAColor{ 255, 255,   0, 255}},	// square
	Block{ 2, [3]Point{Point{ 0,-1}, Point{ 1, 0}, Point{ 1, 1}}, image.RGBAColor{ 255,   0, 255, 255}},	// key1
	Block{ 2, [3]Point{Point{ 1,-1}, Point{ 1, 0}, Point{ 0, 1}}, image.RGBAColor{   0, 255,   0, 255}},	// key2
	Block{ 4, [3]Point{Point{ 0,-1}, Point{ 0, 1}, Point{-1, 1}}, image.RGBAColor{   0,   0, 255, 255}},	// L1
	Block{ 4, [3]Point{Point{-1,-1}, Point{ 0,-1}, Point{ 0, 1}}, image.RGBAColor{ 255, 128,   0, 255}},	// L2
	Block{ 4, [3]Point{Point{-1, 0}, Point{ 0, 1}, Point{ 0,-1}}, image.RGBAColor{   0, 128, 255, 255}}}	// T

/*
0000
0000
0100
0000
*/

func main(){
	var error os.Error
	runtime.GOMAXPROCS(2)
	game := initGame()
	game.context, error = x11.NewWindow()
	if error != nil {
		fmt.Fprintf(os.Stderr, "x11失敗 %s\n", error)
		os.Exit(1)
	}
	// 初期設定
	sync := make(chan bool)
	game.img = game.context.Screen()

	game.createBlock()
	game.putBlock(game.current, false)

	go func(){
		// チャネル入力があるまでストップ
		<-game.context.QuitChan()
		os.Exit(0)
	}()

	for {
		go game.mouseEvent()
		go game.keyboardEvent()
		go game.timerEvent()
		<-sync
	}
}

func (this *Game) mouseEvent(){
	for {
		mouse := <-this.context.MouseChan()
		if mouse.Buttons != 0 {
			// バックアップ
			n := this.current
			if (mouse.Buttons & 0x01) == 0x01 {
				// 左クリック検知
				n.x--
			} else if (mouse.Buttons & 0x02) == 0x02 {
				// 中クリック検知
				n.rotate++
			} else if (mouse.Buttons & 0x04) == 0x04 {
				// 右クリック検知
				n.x++
			}
			if n.x != this.current.x || n.y != this.current.y || n.rotate != this.current.rotate {
				this.deleteBlock(this.current)
				if this.putBlock(n, false) {
					this.current = n
				} else {
					this.putBlock(this.current, false)
				}
				this.printBoard()
				this.context.FlushImage()
			}
		}
	}
}

func (this *Game) keyboardEvent(){
	for {
		key := <-this.context.KeyboardChan()
		if key > 0 {
			n := this.current
			this.deleteBlock(this.current)
			n.y++
			if this.putBlock(n, false) {
				this.current = n
				this.printBoard()
				this.context.FlushImage()
			}
		}
	}
}

func (this *Game) timerEvent(){
	// 500ms
	timer := time.Tick(500 * 1000 * 1000)
	for {
		<-timer
		n := this.current
		this.deleteBlock(n)
		n.y++
		if this.putBlock(n, false) {
			this.current = n
		} else {
			this.putBlock(this.current, false)
			this.createBlock()
			if !this.putBlock(this.current, false) {
				this.gameOver()
				break
			}
		}
		this.printBoard()
		this.context.FlushImage()
	}
	close(timer)
}

func (this *Game) gameOver(){
	for y := 0; y < 25; y++ {
		for x := 0; x < 12; x++ {
			if this.board[y][x] != 0 {
				this.board[y][x] = 1
			}
		}
	}
}

func initGame() (*Game) {
	obj := new(Game)
	obj.random = rand.New(rand.NewSource(time.Nanoseconds()))
	for y := 0; y < 25; y++ {
		for x := 0; x < 12; x++ {
			if x == 0 || x == 11 || y > 21 {
				obj.board[y][x] = 1
			} else {
				obj.board[y][x] = 0
			}
		}
	}
	return obj
}

func (this *Game) createBlock(){
	for y := 1; y < 22; y++ {
		flag := true
		for x := 1; x < 11; x++ {
			if this.board[y][x] == 0 {
				flag = false
			}
		}
		if flag {
			for i := y; i > 1; i-- {
				for j := 1; j < 11; j++ {
					this.board[i][j] = this.board[i - 1][j]
				}
			}
			y--
		}
	}
	this.current.y = 1
	this.current.x = 5
	this.current.ty = this.random.Intn(7) + 1
	this.current.rotate = 0
}

func (this *Game) putBlock(s Status, action bool) (bool){
	if this.board[s.y][s.x] != 0 {
		return false
	}
	// actionがtrueの際に実際に置く
	if action {
		this.board[s.y][s.x] = s.ty
	}
	for i := 0; i < 3; i++ {
		dx := block[s.ty].p[i].x
		dy := block[s.ty].p[i].y
		r := s.rotate % block[s.ty].rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		if this.board[s.y + dy][s.x + dx] != 0 {
			return false
		}
		if action {
			this.board[s.y + dy][s.x + dx] = s.ty
		}
	}
	if !action {
		this.putBlock(s, true)
	}
	return true
}

func (this *Game) deleteBlock(s Status){
	this.board[s.y][s.x] = 0
	for i := 0; i < 3; i++ {
		dx := block[s.ty].p[i].x
		dy := block[s.ty].p[i].y
		r := s.rotate % block[s.ty].rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		this.board[s.y + dy][s.x + dx] = 0
	}
}

func (this *Game) printBoard(){
	white := image.RGBAColor{ 255, 255, 255, 255}
	for y := 0; y < (MASU_Y * BLOCK_PIXEL); y++ {
		for x := 0; x < (MASU_X * BLOCK_PIXEL); x++ {
			dy := (y / BLOCK_PIXEL) + 2
			dx := (x / BLOCK_PIXEL) + 1
			if this.board[dy][dx] != 0 {
				this.img.Set(x, y, block[this.board[dy][dx]].color)
			} else {
				this.img.Set(x, y, white)
			}
		}
	}
}

