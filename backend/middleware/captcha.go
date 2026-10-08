package middleware

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"
	"time"
)

// CaptchaCode 验证码结构
type CaptchaCode struct {
	Code     string
	ExpireAt time.Time
}

// CaptchaManager 验证码管理器
type CaptchaManager struct {
	sync.RWMutex
	codes   map[string]*CaptchaCode
	timeout time.Duration
	length  int
}

var (
	defaultCaptchaManager *CaptchaManager
	once                  sync.Once
)

// InitCaptcha 初始化验证码管理器
func InitCaptcha(length int, timeout time.Duration) {
	once.Do(func() {
		if length <= 0 {
			length = 4
		}
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}
		defaultCaptchaManager = &CaptchaManager{
			codes:   make(map[string]*CaptchaCode),
			timeout: timeout,
			length:  length,
		}
		// 启动清理 goroutine
		go defaultCaptchaManager.cleanup()
	})
}

// GetCaptchaManager 获取默认验证码管理器
func GetCaptchaManager() *CaptchaManager {
	if defaultCaptchaManager == nil {
		InitCaptcha(4, 5*time.Minute)
	}
	return defaultCaptchaManager
}

// cleanup 定期清理过期验证码
func (m *CaptchaManager) cleanup() {
	ticker := time.NewTicker(m.timeout)
	for range ticker.C {
		m.Lock()
		now := time.Now()
		for key, code := range m.codes {
			if now.After(code.ExpireAt) {
				delete(m.codes, key)
			}
		}
		m.Unlock()
	}
}

// Generate 生成验证码，返回 captcha ID 和图片 base64
func (m *CaptchaManager) Generate() (string, string, error) {
	// 生成随机字符
	charSet := "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	code := make([]byte, m.length)
	if _, err := rand.Read(code); err != nil {
		return "", "", err
	}
	for i := range code {
		code[i] = charSet[int(code[i])%len(charSet)]
	}
	codeStr := string(code)

	// 生成 captcha ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", err
	}
	captchaID := base64.URLEncoding.EncodeToString(idBytes)

	// 存储
	m.Lock()
	m.codes[captchaID] = &CaptchaCode{
		Code:     codeStr,
		ExpireAt: time.Now().Add(m.timeout),
	}
	m.Unlock()

	// 生成图片
	img, err := m.renderImage(codeStr)
	if err != nil {
		return "", "", err
	}

	// 编码为 base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	imgBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	return captchaID, imgBase64, nil
}

// Verify 验证验证码，返回是否正确
func (m *CaptchaManager) Verify(captchaID, userInput string) bool {
	m.Lock()
	code, exists := m.codes[captchaID]
	if exists {
		delete(m.codes, captchaID)
	}
	m.Unlock()

	if !exists || code == nil {
		return false
	}

	if time.Now().After(code.ExpireAt) {
		return false
	}

	// 不区分大小写
	return strings.EqualFold(code.Code, userInput)
}

// renderImage 渲染验证码图片
func (m *CaptchaManager) renderImage(code string) (*image.RGBA, error) {
	const (
		dotSize = 7
		charW   = 5 * dotSize
		charH   = 7 * dotSize
		gap     = 12
		padX    = 18
		padY    = 16
	)
	chars := []rune(code)
	width := padX*2 + len(chars)*charW
	if len(chars) > 1 {
		width += (len(chars) - 1) * gap
	}
	height := padY*2 + charH

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 背景色
	bg := color.RGBA{R: 245, G: 247, B: 250, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bg)
		}
	}

	// 干扰线
	for i := 0; i < 6; i++ {
		color := color.RGBA{
			R: uint8(randInt(150, 220)),
			G: uint8(randInt(150, 220)),
			B: uint8(randInt(150, 220)),
			A: 255,
		}
		x0 := randInt(0, width)
		y0 := randInt(0, height)
		x1 := randInt(0, width)
		y1 := randInt(0, height)
		drawLine(img, x0, y0, x1, y1, color)
	}

	// 干扰点
	for i := 0; i < 40; i++ {
		color := color.RGBA{
			R: uint8(randInt(150, 220)),
			G: uint8(randInt(150, 220)),
			B: uint8(randInt(150, 220)),
			A: 255,
		}
		x := randInt(0, width)
		y := randInt(0, height)
		img.Set(x, y, color)
	}

	// 绘制字符，整体放在图片中间
	contentW := len(chars)*charW
	if len(chars) > 1 {
		contentW += (len(chars) - 1) * gap
	}
	originX := (width - contentW) / 2
	originY := (height - charH) / 2
	for i, ch := range chars {
		charColor := color.RGBA{
			R: uint8(randInt(30, 100)),
			G: uint8(randInt(80, 160)),
			B: uint8(randInt(150, 220)),
			A: 255,
		}
		x := originX + i*(charW+gap) + randInt(-2, 3)
		y := originY + randInt(-2, 3)
		m.drawChar(img, x, y, ch, charColor, float64(dotSize*12))
	}

	return img, nil
}

// drawChar 绘制单个字符（用色块模拟）
func (m *CaptchaManager) drawChar(img *image.RGBA, x, y int, ch rune, c color.RGBA, fontSize float64) {
	// 简单的 5x7 像素字体（数字和大写字母）
	patterns := getCharPattern(ch)

	// 基础点大小
	dotSize := int(fontSize / 12)
	if dotSize < 2 {
		dotSize = 2
	}

	for py := 0; py < 7; py++ {
		for px := 0; px < 5; px++ {
			if patterns[py]&(1<<(4-px)) != 0 {
				// 绘制方块
				for dy := 0; dy < dotSize; dy++ {
					for dx := 0; dx < dotSize; dx++ {
						px := x + px*dotSize + dx
						py := y + py*dotSize + dy
						if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
							img.Set(px, py, c)
						}
					}
				}
			}
		}
	}
}

// getCharPattern 获取字符的像素模式
func getCharPattern(ch rune) [7]uint8 {
	// 5x7 的像素图，每个字符用7个字节表示（每字节代表一行5个像素）
	switch ch {
	case 'A':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001}
	case 'B':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10001, 0b10001, 0b11110}
	case 'C':
		return [7]uint8{0b01111, 0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b01111}
	case 'D':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b11110}
	case 'E':
		return [7]uint8{0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b11111}
	case 'F':
		return [7]uint8{0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b10000}
	case 'G':
		return [7]uint8{0b01111, 0b10000, 0b10000, 0b10111, 0b10001, 0b10001, 0b01111}
	case 'H':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001}
	case 'J':
		return [7]uint8{0b00111, 0b00001, 0b00001, 0b00001, 0b00001, 0b10001, 0b01110}
	case 'K':
		return [7]uint8{0b10001, 0b10010, 0b10100, 0b11000, 0b10100, 0b10010, 0b10001}
	case 'L':
		return [7]uint8{0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b11111}
	case 'M':
		return [7]uint8{0b10001, 0b11011, 0b10101, 0b10001, 0b10001, 0b10001, 0b10001}
	case 'N':
		return [7]uint8{0b10001, 0b11001, 0b10101, 0b10011, 0b10001, 0b10001, 0b10001}
	case 'P':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10000, 0b10000, 0b10000}
	case 'Q':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b10001, 0b10101, 0b10010, 0b01101}
	case 'R':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10100, 0b10010, 0b10001}
	case 'T':
		return [7]uint8{0b11111, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100}
	case 'U':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110}
	case 'V':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01010, 0b00100}
	case 'W':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10001, 0b10101, 0b11011, 0b10001}
	case 'X':
		return [7]uint8{0b10001, 0b10001, 0b01010, 0b00100, 0b01010, 0b10001, 0b10001}
	case 'Y':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b01010, 0b00100, 0b00100, 0b00100}
	case 'a':
		return [7]uint8{0b00000, 0b00000, 0b01110, 0b00001, 0b01111, 0b10001, 0b01111}
	case 'b':
		return [7]uint8{0b10000, 0b10000, 0b10110, 0b11001, 0b10001, 0b10001, 0b11110}
	case 'c':
		return [7]uint8{0b00000, 0b00000, 0b01110, 0b10000, 0b10000, 0b10000, 0b01110}
	case 'd':
		return [7]uint8{0b00001, 0b00001, 0b01101, 0b10011, 0b10001, 0b10001, 0b01111}
	case 'e':
		return [7]uint8{0b00000, 0b00000, 0b01110, 0b10001, 0b11110, 0b10000, 0b01111}
	case 'f':
		return [7]uint8{0b00111, 0b01000, 0b01000, 0b11100, 0b01000, 0b01000, 0b01000}
	case 'j':
		return [7]uint8{0b00011, 0b00001, 0b00001, 0b00001, 0b00001, 0b10001, 0b01110}
	case 'm':
		return [7]uint8{0b00000, 0b00000, 0b11010, 0b10101, 0b10101, 0b10001, 0b10001}
	case 'n':
		return [7]uint8{0b00000, 0b00000, 0b11100, 0b10001, 0b10001, 0b10001, 0b10001}
	case 'p':
		return [7]uint8{0b00000, 0b00000, 0b11110, 0b10001, 0b11110, 0b10000, 0b10000}
	case 'q':
		return [7]uint8{0b00000, 0b00000, 0b01101, 0b10011, 0b01111, 0b00001, 0b00001}
	case 'r':
		return [7]uint8{0b00000, 0b00000, 0b10110, 0b10001, 0b10000, 0b10000, 0b10000}
	case 's':
		return [7]uint8{0b00000, 0b00000, 0b01110, 0b10000, 0b01100, 0b00010, 0b11100}
	case 't':
		return [7]uint8{0b01000, 0b01000, 0b11100, 0b01000, 0b01000, 0b01001, 0b00110}
	case 'u':
		return [7]uint8{0b00000, 0b00000, 0b10001, 0b10001, 0b10001, 0b10011, 0b01101}
	case 'v':
		return [7]uint8{0b00000, 0b00000, 0b10001, 0b10001, 0b10001, 0b01010, 0b00100}
	case 'w':
		return [7]uint8{0b00000, 0b00000, 0b10001, 0b10001, 0b10101, 0b11011, 0b10001}
	case 'x':
		return [7]uint8{0b00000, 0b00000, 0b10001, 0b01010, 0b00100, 0b01010, 0b10001}
	case 'y':
		return [7]uint8{0b00000, 0b00000, 0b10001, 0b10001, 0b01111, 0b00001, 0b01110}
	case '2':
		return [7]uint8{0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111}
	case '3':
		return [7]uint8{0b11110, 0b00001, 0b00001, 0b01110, 0b00001, 0b00001, 0b11110}
	case '4':
		return [7]uint8{0b00010, 0b00100, 0b01000, 0b11111, 0b00010, 0b00010, 0b00010}
	case '5':
		return [7]uint8{0b11111, 0b10000, 0b10000, 0b11110, 0b00001, 0b00001, 0b11110}
	case '6':
		return [7]uint8{0b01110, 0b10000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110}
	case '7':
		return [7]uint8{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000}
	case '8':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110}
	case '9':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00001, 0b01110}
	default:
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110}
	}
}

// randInt 生成指定范围内的随机整数
func randInt(min, max int) int {
	if max <= min {
		return min
	}
	n := max - min
	r := int(math.Abs(float64(randIntInt() % n)))
	return min + r
}

// randIntInt 生成随机整数（避免导入 math/rand）
func randIntInt() int {
	b := make([]byte, 4)
	rand.Read(b)
	return int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
}

// drawLine 绘制线段
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy

	for {
		if x0 >= 0 && x0 < img.Bounds().Dx() && y0 >= 0 && y0 < img.Bounds().Dy() {
			img.Set(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
