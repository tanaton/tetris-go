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
	Y		int
	X		int
}

type Block struct {
	Rotate		int
	P			[3]Point
	Color		image.RGBAColor
}

type Status struct {
	Y		int
	X		int
	Ty		int
	Rotate	int
}

type Game struct {
	Current		Status
	Random		*rand.Rand
	Img			draw.Image
	Context		draw.Window
	Reset		chan bool
	Start		bool
	Board		[25][12]int
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
	game.Reset = make(chan bool)
	game.Context, error = x11.NewWindow()
	if error != nil {
		fmt.Fprintf(os.Stderr, "x11失敗 %s\n", error)
		os.Exit(1)
	}
	game.Img = game.Context.Screen()
	game.Start = false
	
	// スレッド実行
	runtime.GOMAXPROCS(4)

	go func(){
		ch := game.Context.EventChan()
		for {
			// チャネル入力があるまでストップ
			i := <- ch
			switch i.(type) {
			case draw.MouseEvent:
				// マウスイベント待ち受け
				data, _ := i.(draw.MouseEvent)
				game.mouseEvent(data)
			case draw.KeyEvent:
				// キーボードイベント待ち受け
				data, _ := i.(draw.KeyEvent)
				game.keyboardEvent(data)
			case draw.ConfigEvent:
			case draw.ErrEvent:
				os.Exit(0)
			}
		}
	}()
	// タイマイベント待ち受け
	go game.timerEvent()

	for {
		// 初期設定
		initGame(game)
		game.createBlock()
		game.putBlock(game.Current, false)
		game.printBoard()
		game.Start = true
		<-game.Reset
	}
	close(game.Reset)
}

func (this *Game) mouseEvent(mouse draw.MouseEvent){
	if mouse.Buttons != 0 && this.Start {
		// バックアップ
		n := this.Current
		if (mouse.Buttons & 0x01) == 0x01 {
			// 左クリック検知
			n.X--
		} else if (mouse.Buttons & 0x02) == 0x02 {
			// 中クリック検知
			n.Rotate++
		} else if (mouse.Buttons & 0x04) == 0x04 {
			// 右クリック検知
			n.X++
		}
		if n.X != this.Current.X || n.Y != this.Current.Y || n.Rotate != this.Current.Rotate {
			this.moveBlock(n)
		}
	}
}

func (this *Game) keyboardEvent(key draw.KeyEvent){
	if key.Key > 0 && this.Start {
		n := this.Current
		switch key.Key {
		case 65361: // left
			n.X--
		case 65362: // down
			n.Rotate++
		case 65363: // right
			n.X++
		case 65364: // up
			n.Y++
		default:
		}
		this.moveBlock(n)
	}
}

func (this *Game) timerEvent(){
	// 500ms
	timer := time.Tick(500 * 1000 * 1000)
	for {
		data := <-timer
		if data == 0 { return }
		if !this.Start { continue }
		n := this.Current
		n.Y++
		if !this.moveBlock(n) {
			this.deleteLine()
			this.createBlock()
			if !this.putBlock(this.Current, false) {
				this.Start = false
				go this.gameOver()
				continue
			}
			this.printBoard()
		}
	}
	close(timer)
}

func (this *Game) moveBlock(n Status) (ret bool){
	this.deleteBlock(this.Current)
	if this.putBlock(n, false) {
		this.Current = n
		ret = true
	} else {
		this.putBlock(this.Current, false)
		ret = false
	}
	this.printBoard()
	return
}

func (this *Game) gameOver(){
	for Y := LIMIT_Y - 1; Y >= 0; Y-- {
		for X := 0; X < BOARD_MAX_X; X++ {
			if this.Board[Y][X] >= 0 {
				this.Board[Y][X] = 0
			}
		}
		this.printBoard()
		// 300ms止める
		time.Sleep(100 * 1000 * 1000)
	}
	this.Reset <- true
}

func initGame(obj *Game) (*Game) {
	obj.Random = rand.New(rand.NewSource(time.Nanoseconds()))
	for Y := 0; Y < BOARD_MAX_Y; Y++ {
		for X := 0; X < BOARD_MAX_X; X++ {
			if X == 0 || X == LIMIT_X || Y >= LIMIT_Y {
				obj.Board[Y][X] = 0
			} else {
				obj.Board[Y][X] = -1
			}
		}
	}
	return obj
}

func (this *Game) createBlock(){
	this.Current.Y = 1
	this.Current.X = 5
	this.Current.Ty = this.Random.Intn(7) + 1
	this.Current.Rotate = 0
}

func (this *Game) deleteLine(){
	for Y := MARGIN_Y; Y < LIMIT_Y; Y++ {
		flag := true
		for X := MARGIN_X; X < LIMIT_X; X++ {
			if this.Board[Y][X] < 0 {
				flag = false
			}
		}
		if flag {
			for i := Y; i > 1; i-- {
				for j := MARGIN_X; j < LIMIT_X; j++ {
					this.Board[i][j] = this.Board[i - 1][j]
				}
			}
			Y--
		}
	}
	this.printBoard()
}

func (this *Game) putBlock(s Status, action bool) (bool){
	if this.Board[s.Y][s.X] >= 0 {
		return false
	}
	// actionがtrueの際に実際に置く
	if action {
		this.Board[s.Y][s.X] = s.Ty
	}
	for i := 0; i < ROTATE_CHANGE; i++ {
		dx := block[s.Ty].P[i].X
		dy := block[s.Ty].P[i].Y
		r := s.Rotate % block[s.Ty].Rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		if this.Board[s.Y + dy][s.X + dx] >= 0 {
			return false
		}
		if action {
			this.Board[s.Y + dy][s.X + dx] = s.Ty
		}
	}
	if !action {
		this.putBlock(s, true)
	}
	return true
}

func (this *Game) deleteBlock(s Status){
	this.Board[s.Y][s.X] = -1
	for i := 0; i < ROTATE_CHANGE; i++ {
		dx := block[s.Ty].P[i].X
		dy := block[s.Ty].P[i].Y
		r := s.Rotate % block[s.Ty].Rotate
		for j := 0; j < r; j++ {
			nx, ny := dx, dy
			dx, dy = ny, -nx
		}
		this.Board[s.Y + dy][s.X + dx] = -1
	}
}

func (this *Game) printBoard(){
	white := image.RGBAColor{ 0xFF, 0xFF, 0xFF, 0xFF}
	for Y := 0; Y < (MASU_Y * BLOCK_PIXEL); Y++ {
		for X := 0; X < (MASU_X * BLOCK_PIXEL); X++ {
			dy := (Y / BLOCK_PIXEL) + MARGIN_Y
			dx := (X / BLOCK_PIXEL) + MARGIN_X
			if this.Board[dy][dx] >= 0 {
				this.Img.Set(X, Y, block[this.Board[dy][dx]].Color)
			} else {
				this.Img.Set(X, Y, white)
			}
		}
	}
	this.Context.FlushImage()
}

