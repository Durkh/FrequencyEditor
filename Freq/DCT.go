package Freq

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

type Wave interface {
	DCT(map[string]interface{}) *Frequencies
	IDCT(*Frequencies) Wave
	Open(string) error
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
		bounds = limits{
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

func (f *Frequencies) ApplyFilter(cf, order int) {

	var (
		distance = func(x, y int) float64 {
			return math.Sqrt(float64(x*x + y*y))
		}

		butterworth = func(x, y int) {
			f.Data2D[y][x] *= 1 / (math.Sqrt(1 + math.Pow(distance(x, y)/float64(cf), float64(2*order))))
		}
	)

	iterate(limits{X: len(f.Data2D[0]), Y: len(f.Data2D)}, butterworth)

}

type limits = struct {
	X, Y int
}

func iterate(bounds limits, closure func(int, int)) {

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
