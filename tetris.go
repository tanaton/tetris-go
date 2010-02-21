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
	reset		chan bool
	start		bool
	board		[25][12]int
}

const (
	MASU_X				= 10
	MASU_Y				= 20
	BOARD_MAX_X			= 12
	BOARD_MAX_Y			= 25
	MARGIN_X			= 1
	MARGIN_Y			= 2
	LIMIT_X				= 11
	LIMIT_Y				= 22
	ROTATE_CHANGE		= 3
	BLOCK_PIXEL			= 24
	BLOCK_END_TYPE		= 8
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

func main(){
	var error os.Error
	game := new(Game)
	game.reset = make(chan bool)
	game.context, error = x11.NewWindow()
	if error != nil {
		fmt.Fprintf(os.Stderr, "x11失敗 %s\n", error)
		os.Exit(1)
	}
	game.img = game.context.Screen()
	game.start = false
	
	// スレッド実行
	runtime.GOMAXPROCS(2)

	go func(){
		// チャネル入力があるまでストップ
		<-game.context.QuitChan()
		os.Exit(0)
	}()

	// マウスイベント待ち受け
	go game.mouseEvent()
	// キーボードイベント待ち受け
	go game.keyboardEvent()
	// タイマイベント待ち受け
	go game.timerEvent()

	for {
		// 初期設定
		initGame(game)
		game.createBlock()
		game.putBlock(game.current, false)
		game.printBoard()
		game.start = true
		<-game.reset
	}
	close(game.reset)
}

func (this *Game) mouseEvent(){
	for {
		mouse := <-this.context.MouseChan()
		if mouse.Buttons != 0 && this.start {
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
				this.moveBlock(n)
			}
		}
	}
}

func (this *Game) keyboardEvent(){
	for {
		key := <-this.context.KeyboardChan()
		if key > 0 && this.start {
			n := this.current
			n.y++
			this.moveBlock(n)
		}
	}
}

func (this *Game) timerEvent(){
	// 500ms
	timer := time.Tick(500 * 1000 * 1000)
	for {
		<-timer
		if !this.start { continue }
		n := this.current
		n.y++
		if !this.moveBlock(n) {
			this.deleteLine()
			this.createBlock()
			if !this.putBlock(this.current, false) {
				this.start = false
				go this.gameOver()
				continue
			}
			this.printBoard()
		}
	}
	close(timer)
}

func (this *Game) moveBlock(n Status) (ret bool){
	this.deleteBlock(this.current)
	if this.putBlock(n, false) {
		this.current = n
		ret = true
	} else {
		this.putBlock(this.current, false)
		ret = false
	}
	this.printBoard()
	return
}

func (this *Game) gameOver(){
	for y := LIMIT_Y - 1; y >= 0; y-- {
		for x := 0; x < BOARD_MAX_X; x++ {
			if this.board[y][x] >= 0 {
				this.board[y][x] = 0
			}
		}
		this.printBoard()
		// 300ms止める
		time.Sleep(100 * 1000 * 1000)
	}
	this.reset <- true
}

func initGame(obj *Game) (*Game) {
	obj.random = rand.New(rand.NewSource(time.Nanoseconds()))
	for y := 0; y < BOARD_MAX_Y; y++ {
		for x := 0; x < BOARD_MAX_X; x++ {
			if x == 0 || x == LIMIT_X || y >= LIMIT_Y {
				obj.board[y][x] = 0
			} else {
				obj.board[y][x] = -1
			}
		}
	}
	return obj
}

func (this *Game) createBlock(){
	this.current.y = 1
	this.current.x = 5
	this.current.ty = this.random.Intn(7) + 1
	this.current.rotate = 0
}

func (this *Game) deleteLine(){
	for y := MARGIN_Y; y < LIMIT_Y; y++ {
		flag := true
		for x := MARGIN_X; x < LIMIT_X; x++ {
			if this.board[y][x] < 0 {
				flag = false
			}
		}
		if flag {
			for i := y; i > 1; i-- {
				for j := MARGIN_X; j < LIMIT_X; j++ {
					this.board[i][j] = this.board[i - 1][j]
				}
			}
			y--
		}
	}
	this.printBoard()
}

func (this *Game) putBlock(s Status, action bool) (bool){
	if this.board[s.y][s.x] >= 0 {
		return false
	}
	// actionがtrueの際に実際に置く
	if action {
		this.board[s.y][s.x] = s.ty
	}
	for i := 0; i < ROTATE_CHANGE; i++ {
		dx := block[s.ty].p[i].x
		dy := block[s.ty].p[i].y
		r := s.rotate % block[s.ty].rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		if this.board[s.y + dy][s.x + dx] >= 0 {
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
	this.board[s.y][s.x] = -1
	for i := 0; i < ROTATE_CHANGE; i++ {
		dx := block[s.ty].p[i].x
		dy := block[s.ty].p[i].y
		r := s.rotate % block[s.ty].rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		this.board[s.y + dy][s.x + dx] = -1
	}
}

func (this *Game) printBoard(){
	white := image.RGBAColor{ 0xFF, 0xFF, 0xFF, 0xFF}
	for y := 0; y < (MASU_Y * BLOCK_PIXEL); y++ {
		for x := 0; x < (MASU_X * BLOCK_PIXEL); x++ {
			dy := (y / BLOCK_PIXEL) + MARGIN_Y
			dx := (x / BLOCK_PIXEL) + MARGIN_X
			if this.board[dy][dx] >= 0 {
				this.img.Set(x, y, block[this.board[dy][dx]].color)
			} else {
				this.img.Set(x, y, white)
			}
		}
	}
	this.context.FlushImage()
}

