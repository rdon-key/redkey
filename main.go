package main

import (
	"image/color"
	"machine"
	"strconv"
	"time"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/shnm"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
)


type WS2812B struct {
	Pin machine.Pin
	ws  *piolib.WS2812B
}

func NewWS2812B(pin machine.Pin) *WS2812B {
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, pin)
	ws.EnableDMA(true)
	return &WS2812B{
		ws: ws,
	}
}

func (ws *WS2812B) PutColor(c color.Color) {
	ws.ws.PutColor(c)
}

func (ws *WS2812B) WriteRaw(rawGRB []uint32) error {
	return ws.ws.WriteRaw(rawGRB)
}



const (
	testDuration = 10 * time.Second
	ROWS        = 3
	COLS        = 4
)

// キーの状態を保持する構造体
type KeyState struct {
	isPressed bool
	lastPress time.Time
}

func main() {



	ws := NewWS2812B(machine.GPIO1)
	
	// 12個のLEDすべてを制御する配列を作成（最初はすべて消灯）
	leds := make([]uint32, 12)
	
	// 右下のLED（12番目）のみを赤く点灯
	// WS2812Bのカラーフォーマットは GRB なので、赤は 0x00FF0000
	leds[11] = 0x00FF0000
	
	// LEDに書き込み
	ws.WriteRaw(leds)
	// I2Cの設定
	machine.I2C0.Configure(machine.I2CConfig{
		Frequency: 2.8 * machine.MHz,
		SDA:       machine.GPIO12,
		SCL:       machine.GPIO13,
	})

	// ディスプレイの初期化
	display := ssd1306.NewI2C(machine.I2C0)
	display.Configure(ssd1306.Config{
		Address: 0x3C,
		Width:   128,
		Height:  64,
	})
	display.SetRotation(drivers.Rotation180)
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	// キーマトリックスの設定
	colPins := []machine.Pin{
		machine.GPIO5,
		machine.GPIO6,
		machine.GPIO7,
		machine.GPIO8,
	}
	rowPins := []machine.Pin{
		machine.GPIO9,
		machine.GPIO10,
		machine.GPIO11,
	}

	// ピンの初期化
	for _, c := range colPins {
		c.Configure(machine.PinConfig{Mode: machine.PinOutput})
		c.Low()
	}
	for _, c := range rowPins {
		c.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}

	// キー状態の初期化
	keyStates := make([][]KeyState, ROWS)
	for i := range keyStates {
		keyStates[i] = make([]KeyState, COLS)
	}

	for {
		display.ClearDisplay()
		tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 12, "赤いキーを連打！！", white)
		display.Display()

		// キー待機
		waitForSW12Key(colPins, rowPins)

		// タイピングテスト開始
		keyCount := 0
		startTime := time.Now()
		endTime := startTime.Add(testDuration)

		// キー状態をリセット
		for i := range keyStates {
			for j := range keyStates[i] {
				keyStates[i][j].isPressed = false
			}
		}

		for time.Now().Before(endTime) {
			keyCount += scanKeys(colPins, rowPins, &keyStates)
			
			display.ClearDisplay()
			remainingTime := int(endTime.Sub(time.Now()).Seconds())
			tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 12, "残り時間: "+strconv.Itoa(remainingTime)+"秒", white)
			tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 30, "打鍵数: "+strconv.Itoa(keyCount), white)
			display.Display()
		}

		// 結果表示
		display.ClearDisplay()
		tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 12, "テスト終了!", white)
		tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 30, "合計打鍵数: "+strconv.Itoa(keyCount), white)
		speed := float64(keyCount) / 10.0
		speedStr := strconv.FormatFloat(speed, 'f', 1, 64)
		tinyfont.WriteLine(&display, &shnm.Shnmk12, 5, 48, "速度: "+speedStr+"回/秒", white)
		display.Display()

		time.Sleep(3 * time.Second)
	}
}

func waitForSW12Key(colPins []machine.Pin, rowPins []machine.Pin) {
	wasPressed := false
	for {
		colPins[3].High()
		time.Sleep(1 * time.Millisecond)
		
		if rowPins[2].Get() {
			if !wasPressed {
				wasPressed = true
			}
		} else if wasPressed {
			break // キーが離されたときに抜ける
		}
		
		colPins[3].Low()
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
}

func scanKeys(colPins []machine.Pin, rowPins []machine.Pin, keyStates *[][]KeyState) int {
	keyCount := 0
	now := time.Now()
	
	for col := 0; col < len(colPins); col++ {
		// 現在の列をアクティブに
		for i, pin := range colPins {
			if i == col {
				pin.High()
			} else {
				pin.Low()
			}
		}
		
		time.Sleep(1 * time.Millisecond)
		
		// 行をスキャン
		for row := 0; row < len(rowPins); row++ {
			currentState := rowPins[row].Get()
			
			// キーが押されている
			if currentState {
				if !(*keyStates)[row][col].isPressed {
					// キーが新しく押された
					(*keyStates)[row][col].isPressed = true
					(*keyStates)[row][col].lastPress = now
					keyCount++
				}
			} else {
				// キーが離された
				if (*keyStates)[row][col].isPressed {
					(*keyStates)[row][col].isPressed = false
					// デバウンス時間（必要に応じて調整）
					if now.Sub((*keyStates)[row][col].lastPress) > 50*time.Millisecond {
						// ここでは追加のアクションは不要
					}
				}
			}
		}
	}
	
	return keyCount
}

