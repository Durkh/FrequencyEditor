package Freq

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

type Wave interface {
	Open(string) error
	DCT(map[string]interface{}) *Frequencies
	IDCT(*Frequencies) Wave
	Save() error
}

type Frequencies struct {
	Data2D   [][]float64
	Filename string
}

func NewFreq(Ysiz, Xsiz int, name ...string) *Frequencies {

	var (
		d = make([][]float64, Ysiz)
		n string
	)

	for i := range d {
		d[i] = make([]float64, Xsiz)
	}

	if len(name) > 0 {
		n = name[0]
	}

	return &Frequencies{d, n}
}

func (f *Frequencies) ToGray() (im *image.Gray) {

	im = image.NewGray(image.Rect(0, 0, len(f.Data2D[0]), len(f.Data2D)))

	var (
		bounds = waveLimits{
			X: im.Bounds().Max.X,
			Y: im.Bounds().Max.Y,
		}
	)

	f.Data2D[0][0] = 0xff

	iterate(bounds, func(x int, y int) {
		im.Set(x, y, color.Gray{Y: uint8(f.Data2D[y][x])})
	})

	return
}

func (f *Frequencies) ApplyFilter(cf, order int, data func(int, int) float64) {

	var (
		butterworth = func(x, y int) {
			var a = 1 / (math.Sqrt(1 + math.Pow(data(x, y)/float64(cf), float64(2*order))))
			f.Data2D[y][x] *= a
		}
	)

	iterate(waveLimits{X: len(f.Data2D[0]), Y: len(f.Data2D)}, butterworth)

}

type waveLimits = struct {
	X, Y int
}

func iterate(bounds waveLimits, closure func(int, int)) {

	var (
		wg sync.WaitGroup

		funcChannel = make(chan image.Point, 64)
		middleman   = func(pixels <-chan image.Point) {
			defer wg.Done()
			for pixel := range pixels {
				closure(pixel.X, pixel.Y)
			}
		}
	)

	//cleanup
	defer wg.Wait()
	defer close(funcChannel)

	//worker pool
	wg.Add(runtime.NumCPU() * 2)
	for i := 0; i < runtime.NumCPU()*2; i++ {
		go middleman(funcChannel)
	}

	//iteration
	for y := 0; y < bounds.Y; y++ {
		for x := 0; x < bounds.X; x++ {

			funcChannel <- image.Point{X: x, Y: y}
		}
	}

}

type Limits struct {
	Low, High int
}

//DCT1d
// k = index
// Xk = color
// ft = DCT's First Term
func DCT1d(k, N int, ft float64, bounds Limits, access func(*int) float64, assign func(*float64), minMaxFunc func(float64)) {

	var (
		pixConst         = math.Pi * float64(k) / float64(N)
		Ck       float64 = 1
		sum, res float64
	)

	if k == 0 {
		Ck = math.Sqrt(1.0 / 2.0)
	}

	//DCT sum
	for n := bounds.Low; n < bounds.High; n++ {
		sum += access(&n) * math.Cos(pixConst*(float64(n)+.5))
	}

	//constant part
	res = ft * Ck * sum
	assign(&res)

	//get image min and max values
	minMaxFunc(math.Abs(res))
}

func IDCT1d(n, N int, ft float64, bounds Limits, access func(*int) float64, assign func(*float64)) {

	var (
		pixConst = (math.Pi / float64(N)) * (float64(n) + .5)
		Ck       = math.Sqrt(1.0 / 2.0)
		sum, res float64
	)

	//DCT sum
	for k := bounds.Low; k < bounds.High; k++ {
		sum += access(&k) * math.Cos(pixConst*float64(k)) * Ck
		Ck = 1
	}

	//constant part
	res = ft * sum
	assign(&res)
}
